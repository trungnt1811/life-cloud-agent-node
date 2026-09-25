# Node Agent Connection Guide

Audience: engineers operating or extending the control plane's `NodeControl`
gRPC server. This describes behavior implemented in the node agent
(`life-cloud-agent-node`). Code and tests cited below provide the current
source of truth.

This repository implements only the **node** side. The sibling
`life-cloud-control-plane` repository implements the server and runs a
cross-repository mTLS integration test against the production agent binary.

## 1. Summary

- The node **dials out**; it is the gRPC **client**. The control center is
  the gRPC **server**. A node never listens for inbound connections for
  federated query traffic (decision 0001).
- One long-lived, bidirectional stream per node:
  `rpc Connect(stream NodeToCenter) returns (stream CenterToNode);`
  (`api/proto/lifecloud/node/v1/node_control.proto`).
- The node reconnects on its own with backoff whenever the stream drops. The
  control center does not need to, and cannot, reconnect to the node.
- All query traffic is **aggregate-only**: a node never returns patient-level
  rows, only a suppressed count.

## 2. Transport and authentication

- **TLS is required by default** (decision 0006). The node verifies your
  server certificate against the system CA pool, or a custom CA file if the
  node operator configures one. There is a node-local opt-out
  (`CONTROL_CENTER_INSECURE=true`) for local development only; do not expect
  production nodes to use it.
- **mTLS is required for production connections.** When `ENV=production` or
  `prod` and `CONTROL_CENTER_ADDRESS` is set, the node requires both client
  certificate and key files and rejects `CONTROL_CENTER_INSECURE=true`.
  The control plane's production gRPC gateway also requires and verifies the
  client certificate against the registered node fingerprint. Development
  setups can omit mTLS. Without certificate verification, `node_id` in
  `Register` is only a self-asserted claim.
- With TLS enabled, an invalid CA file or client key pair stops application
  startup before it opens the local database or serves HTTP. Network outages
  after startup are retried with backoff.
- There is no other wire-level authentication (no API key or bearer token
  field in the proto). If you need stronger node authentication than mTLS
  provides, that is a protocol change, not a configuration one.

## 3. Connection lifecycle

1. The node dials your address and opens the `Connect` stream.
2. The node immediately sends one `Register` message (§4.1). Send nothing
   back until you've recorded it; there is no explicit "connection accepted"
   message.
3. The node sends a `Heartbeat` roughly every 30 seconds while the stream is
   open (§4.2) and expects none in return.
4. You send `QueryTask` (§5) and `UpdateAdvisory` (§6) messages whenever you
   have them; there's no rate limit on your side implemented by the node.
5. If the underlying connection drops for any reason (network blip, your
   server restarting, an explicit close), the node's `stream.Recv()` errors
   out, the node abandons that connection, waits (see §7 for the backoff
   shape), reconnects, and sends a **fresh `Register`**. Treat a new
   `Register` from a `node_id` you already had as "this node reconnected,"
   not as a second node.
6. The node does not send any kind of graceful "goodbye" message before
   disconnecting (process shutdown, etc.) — the stream simply ends. Detect a
   node going away by the stream closing or heartbeats stopping.

## 4. Session messages

### 4.1 `Register` — node → you

```proto
message Register {
  string node_id = 1;
  string agent_version = 2;
  uint32 query_schema_version = 3;
}
```

Sent once immediately on every connect and every reconnect. `node_id` and
`agent_version` are operator-configured strings (`NODE_ID`, `AGENT_VERSION`
env vars) — the node does not validate their format. `query_schema_version`
is the **highest** schema version this node build understands (`1` today;
see §5.6 for what happens when you send a task above that).

Use this to maintain your own node registry (online/offline, last-seen
`agent_version`) — the node has no equivalent read endpoint on the wire; it
is entirely up to you to track this from `Register`/`Heartbeat` traffic.

### 4.2 `Heartbeat` — node → you

```proto
message Heartbeat {
  google.protobuf.Timestamp sent_at = 1;
  repeated string inflight_job_ids = 2;
}
```

Sent every `HeartbeatInterval` while the stream is open — **30 seconds by
default**, but this is a node-local operational default
(`internal/adapters/federated/client.DefaultHeartbeatInterval`), not a
negotiated or wire-guaranteed value; a future node build could run a
different interval. Do not hard-code an exact expected cadence; use a
multiple of whatever interval you observe (e.g. "no heartbeat for 3x the
observed interval ⇒ treat as offline") rather than a fixed threshold.

`inflight_job_ids` is a snapshot of `job_id`s the node is currently
executing (including ones interrupted mid-checkpoint and not yet resumed).
There is no per-job progress percentage — this is presence only.

## 5. Query flow

### 5.1 Sending a task — you → node

```proto
message QueryTask {
  string job_id = 1;
  uint32 query_schema_version = 2;
  DateRange time_range = 3;
  SpecimenPolicy specimen_policy = 4;
  repeated QueryCondition conditions = 5;
  repeated RequiredPanel required_panels = 6;
  repeated string group_by = 7; // reserved; must be empty in v1
}
```

Field-by-field requirements the node enforces (rejects otherwise — see
§5.4):

| Field | Requirement |
|---|---|
| `job_id` | Required, non-empty after trimming whitespace, **at most 255 characters**. This is your idempotency/resume key — see §5.5. |
| `query_schema_version` | Must be `1`. Anything else short-circuits straight to `UNSUPPORTED_VERSION` without touching any other field (§5.6). |
| `time_range.from` / `.to` | Required, `YYYY-MM-DD`, inclusive, `from <= to`. Padded whitespace is rejected, not trimmed. |
| `specimen_policy` | Must be `SPECIMEN_POLICY_LATEST_IN_RANGE` — the only value schema v1 defines. |
| `conditions[].field_code` | Must be one of the 9 schema-v1 codes (`docs/product/query-field-dictionary.md`) **and** enabled in that specific node's local whitelist. A field can be valid platform-wide and still rejected by one node that hasn't opted in. |
| `conditions[].op` | One of `EQ, NE, LT, LTE, GT, GTE`. |
| `conditions[].value.number_value` | A canonical decimal string: `^-?\d+(\.\d+)?$`, ≤ 32 characters. No exponents, hex/binary/octal, `_` separators, leading `+`, or padding. `string_value` is rejected for all v1 fields (all are numeric measurements). |
| `required_panels[].field_codes` | Same whitelist rule as conditions; non-empty. |
| `required_panels[].value_constraint` | `VALUE_CONSTRAINT_EXACT` (every field present, non-censored) or `VALUE_CONSTRAINT_ALLOW_CENSORED` (presence counts even if censored, e.g. HbF `<0.1`). |
| `group_by` | Must be empty. Reserved for a future schema version. |

See `docs/product/query-field-dictionary.md` for the full field list and the
demo cohort rule's exact shape.

### 5.2 Result — node → you

```proto
message QueryResult {
  string job_id = 1;
  QueryResultStatus status = 2;
  string reason = 3;          // required when status != OK
  uint64 matching_count = 4;  // set only when status == OK
  bool suppressed = 5;
}
```

`matching_count` is **always post-suppression**: if the raw match count was
below the node's local small-cell threshold, the node reports `0` with
`suppressed = true` instead of the real (small) number. There is no wire
field for the raw count — a node will never send it, by design.

### 5.3 Status values

| Status | Meaning | Retry? |
|---|---|---|
| `OK` | `matching_count` is valid. | N/A — done. |
| `REJECTED_INVALID_QUERY` | The task failed structural, whitelist, or semantic validation (`reason` says which field/rule). | Only after fixing the task; resending the identical task gets the identical rejection. |
| `UNSUPPORTED_VERSION` | `query_schema_version` isn't `1`. | Only after sending a version this node's `Register` advertised. |
| `ERROR` | Internal node-side failure (e.g. its database is unreachable). | Safe to retry; not the node's fault. |

### 5.4 Validation order

The node validates in this exact order and stops at the first failure
(`internal/adapters/federated/wire/validate.go`):

**version → structural → whitelist (this node's D5 store) → semantic**

Version is checked *first*, ahead of its numeric position in the proto
comments, specifically so a `query_schema_version` this build doesn't
understand never gets judged by v1-specific rules and never costs a
whitelist database read. Don't infer anything about field/whitelist validity
from an `UNSUPPORTED_VERSION` response — the node didn't look that far.

### 5.5 No response is not a final answer — resume semantics

**This is the one behavior most likely to surprise a new integrator.** If
the node's connection to you drops while it is still executing a task (for
a large cohort, execution is internally chunked and checkpointed — decision
0005), the node sends **nothing** for that attempt: not `ERROR`, not a
partial count. It reconnects and waits for you to ask again.

The correct pattern on your side:

1. Send `QueryTask`.
2. Wait for `QueryResult` on that stream, with your own timeout.
3. If the stream drops or your timeout fires before a `QueryResult` arrives,
   **do not assume failure**. Wait for the node's `Register` (it will
   reconnect on its own — see §7) and **resend the identical `QueryTask`
   (same `job_id`, same criteria)**.
4. The node resumes from its last completed checkpoint under that `job_id`
   and returns the correct total — chunks already counted are not
   re-counted, and none are skipped. If the job had already finished before
   you noticed the drop, the resend is served from the stored result without
   re-querying anything.
5. **Never reuse a `job_id` for a materially different query.** The node
   detects a changed query (it hashes the criteria) and discards the old
   checkpoint rather than mixing counts, but that means a job_id collision
   between two different queries silently restarts one of them from zero
   instead of erroring — treat `job_id` as globally unique per query, not
   per node.

If the same `job_id` arrives again while its task is already executing on the
same stream, the node ignores that duplicate. The first execution sends one
result. Resend after reconnect if that result was lost, as described above.

### 5.6 Worked example

A happy path, in message order:

```
node → you   Register{node_id: "hospital-a", agent_version: "1.4.0", query_schema_version: 1}
you  → node  QueryTask{job_id: "job-8f21", query_schema_version: 1,
                        time_range: {from: "2024-01-01", to: "2024-12-31"},
                        specimen_policy: LATEST_IN_RANGE,
                        conditions: [{field_code: "MCV", op: LT, value.number_value: "80"}],
                        required_panels: [{field_codes: ["HB","MCV","MCH","RBC"], value_constraint: EXACT}]}
node → you   QueryResult{job_id: "job-8f21", status: OK, matching_count: 3, suppressed: false}
```

A rejected task (unknown/disabled field):

```
you  → node  QueryTask{job_id: "job-91aa", ..., conditions: [{field_code: "HBF", ...}]}
node → you   QueryResult{job_id: "job-91aa", status: REJECTED_INVALID_QUERY,
                          reason: "conditions[0].field_code \"HBF\" is unknown or not enabled on this node"}
```

A dropped-connection retry (see §5.5):

```
you  → node  QueryTask{job_id: "job-c410", ...}
             (connection drops mid-execution; nothing arrives)
node → you   Register{node_id: "hospital-a", ...}      // reconnect
you  → node  QueryTask{job_id: "job-c410", ...}          // identical resend
node → you   QueryResult{job_id: "job-c410", status: OK, matching_count: 3}
```

## 6. `UpdateAdvisory` — you → node

```proto
message UpdateAdvisory {
  string version = 1;
  string changelog_url = 2;
  AdvisorySeverity severity = 3; // INFO, RECOMMENDED, CRITICAL
}
```

Send this whenever a new node build is available. The node only **logs it
and exposes it locally** via its own admin REST (`GET /admin/status`) for a
human operator to see — it never fetches, verifies, or applies an update
itself (decision 0003). There is no acknowledgment on the wire: you cannot
tell, from the stream alone, whether the node's operator has seen or acted
on an advisory. Applying an update is an out-of-band `docker compose pull &&
up -d` a human runs; expect nodes in your fleet to run different
`agent_version`s at any given time and design your dispatch logic (and
`query_schema_version` expectations) accordingly.

## 7. Reconnection and backoff

On any stream loss, the node retries with jittered exponential backoff:
starts at 1 second, doubles each attempt, capped at 30 seconds, with each
wait randomized to somewhere in the top half of that interval
(`internal/adapters/federated/client.DefaultInitialBackoff` /
`DefaultMaxBackoff`). These are node-local defaults an operator can change;
don't hard-code them into your own timeout logic. Practically: expect a
disconnected node to attempt reconnecting within seconds, and to keep
retrying indefinitely — there is no give-up point.

## 8. Known limitations (v1)

These are real gaps, not just unfinished polish — build your control center
logic around them rather than assuming they'll silently disappear:

- **No wire-level "what can this node answer?" query.** The whitelist is
  entirely node-local (each hospital's own `enabled_query_fields`); the only
  way to learn it today is to send a task and observe
  `REJECTED_INVALID_QUERY`, or have the node operator tell you out of band.
- **`node_id` needs certificate binding.** The production control plane
  verifies the registered client-certificate fingerprint; a development
  server without that check receives only a self-asserted ID (§2).
- **No `UpdateAdvisory` acknowledgment** (§6).
- **One active connection per node is assumed.** The node does not
  coordinate multiple simultaneous streams from the same process; don't
  dial a node yourself; only accept its inbound connection.
- **`group_by` is reserved and always rejected in v1** — no grouped
  aggregates yet, cohort counts only.

## 9. Where this comes from

- `api/proto/lifecloud/node/v1/node_control.proto` — normative wire shapes.
- `docs/decisions/0001-node-agent-role-and-grpc-channel.md` — who's the
  client, who's the server.
- `docs/decisions/0002-federated-query-contract-and-execution.md` — the
  execution/validation model this guide summarizes.
- `docs/decisions/0003-node-update-and-release-process.md` — advisory
  semantics (§6).
- `docs/decisions/0004-grpc-wire-contract-schema-v1.md` — full schema v1
  field-by-field contract.
- `docs/decisions/0005-job-checkpoint-granularity.md` — why resend-by-job_id
  resumes instead of restarts (§5.5).
- `docs/decisions/0006-grpc-client-transport-security.md` — TLS/mTLS (§2).
- `docs/product/federated-query-flow.md` — sequence diagrams (Fig. 3, 4).
- `docs/product/query-field-dictionary.md` — the 9 schema-v1 field codes.
- `internal/adapters/federated/client/node_client.go` — the node's actual
  connection/reconnect/heartbeat implementation this document describes.
