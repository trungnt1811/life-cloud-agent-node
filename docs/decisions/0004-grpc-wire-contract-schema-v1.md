# 0004 gRPC Wire Contract And Query Schema v1

Date: 2026-09-16

## Status

Accepted

## Context

Decisions 0001–0003 fixed the federated role, async structured-query model,
four-layer validation, aggregate-only output, and update advisories, but left
the concrete `.proto` shape as an explicit follow-up. The sibling
`life-cloud` demo fixture `data/federated_mvp/benchmark/query_definition.json`
is **not** the same shape as the earlier prose sketch of
`conditions[{field,op,value}]`: it also encodes specimen selection, required
CBC/HPLC panels, and censored-vs-exact value rules. Writing a `.proto` without
closing those gaps would invent externally observable policy. This decision is
the wire-contract artifact those follow-ups required; it does not implement
generated code.

## Decision

### Stream and service

- Package name for the first contract: `lifecloud.node.v1`.
- One bidirectional stream, **node-initiated** (dial-out), single RPC:

  `rpc Connect(stream NodeToCenter) returns (stream CenterToNode);`

  Service name: `NodeControl`.

- Envelope oneofs (no per-message RPCs in v1):

  | Direction | Allowed messages |
  |---|---|
  | `NodeToCenter` | `Register`, `Heartbeat`, `QueryResult` |
  | `CenterToNode` | `QueryTask`, `UpdateAdvisory` |

### Session messages

- `Register`: `node_id` (string), `agent_version` (string),
  `query_schema_version` (uint32 — highest schema this build understands).
  Sent on every (re)connect.
- `Heartbeat`: `sent_at` (timestamp), optional `inflight_job_ids` (repeated
  string). Interval remains an operational default (30s in the flow docs), not
  a wire field.
- `UpdateAdvisory`: `version` (string), `changelog_url` (string),
  `severity` enum: `INFO`, `RECOMMENDED`, `CRITICAL`. The node only logs and
  exposes locally; it never auto-updates (decision 0003).

### Query schema version 1

`query_schema_version = 1` means the following `QueryTask` shape and
semantics. A node that does not understand the task's version returns
`QueryResult.status = UNSUPPORTED_VERSION` without running anything.

`QueryTask`:

| Field | Meaning |
|---|---|
| `job_id` | Opaque job id from the control center. |
| `query_schema_version` | Must be `1` for this decision. |
| `time_range.from` / `time_range.to` | Inclusive calendar dates (`YYYY-MM-DD` semantics); bound the specimen window. |
| `specimen_policy` | Enum; v1 allows only `LATEST_IN_RANGE` (latest accepted specimen in range, then lexical specimen id as in the demo README). |
| `conditions` | Repeated measurement predicates: `field_code`, `op`, `value`. |
| `required_panels` | Repeated panels: `field_codes[]` plus `value_constraint`. |
| `group_by` | Repeated `field_code`. **v1 nodes must reject** any non-empty `group_by` with `REJECTED_INVALID_QUERY` (no bucket schema yet). |

Operators (`op` enum): `EQ`, `NE`, `LT`, `LTE`, `GT`, `GTE`.

Condition values use a `oneof`: `number_value` (decimal **string**, never
binary float) or `string_value`. MVP measurement rules use `number_value`.

`value_constraint` on a required panel:

- `EXACT` — every listed field must be present on the selected specimen with a
  non-censored exact numeric value (demo CBC rule).
- `ALLOW_CENSORED` — presence counts even when the stored result is censored
  (e.g. HbF `<0.1`; demo HPLC rule).

`field_code` is a **string** on the wire, not a protobuf enum. The global
maximum set for schema v1 lives in
`docs/product/query-field-dictionary.md` and is owned by Life Cloud. Each
node still validates layer-2 whitelist against its local `enabled_query_fields`
store (decision 0002).

### Mapping the demo fixture to a QueryTask

`life-cloud` `query_definition.json` is a **compile input** for tests / the
control center, not a second wire type. Fixture `version` does not travel on
the wire.

| Fixture | Schema v1 wire |
|---|---|
| `date_start` / `date_end` | `time_range` |
| latest-specimen prose | `specimen_policy = LATEST_IN_RANGE` |
| `rule.MCV_lt_fL` / `MCH_lt_pg` | `conditions` with `LT` on `MCV` / `MCH` |
| `cbc_required` | one `required_panels` entry, `value_constraint = EXACT` |
| `hplc_required` | one `required_panels` entry, `value_constraint = ALLOW_CENSORED` |

### QueryResult

| Field | Meaning |
|---|---|
| `job_id` | Same id as the task. |
| `status` | `OK`, `REJECTED_INVALID_QUERY`, `UNSUPPORTED_VERSION`, `ERROR`. |
| `reason` | Human-readable; required when status ≠ `OK`. |
| `matching_count` | Set only when `status = OK`; count after small-cell suppression. |
| `suppressed` | `true` when the raw match count was hidden because it was below the node-local suppression threshold. |

No patient-level rows, sums, or means on the wire in v1. Suppression threshold
is node-local configuration and is not a wire field.

Four-layer validation order and rejection mapping remain as in decision 0002 /
`docs/product/federated-query-flow.md` Fig. 3.

## Alternatives Considered

1. Protobuf enums for every lab `field_code`. Rejected: the global set must
   evolve without forcing a wire break for every new assay code; string codes
   plus a versioned dictionary match decision 0002's ownership split.
2. Treat `query_definition.json` as the wire document. Rejected: it is an
   engineering cohort fixture, not a general federated contract, and omits
   session/advisory messages.
3. Flat `conditions`-only task without panels / specimen policy. Rejected:
   cannot express the accepted demo cohort without silent node-local
   conventions that would not be in the shared contract.
4. Separate unary RPCs per message type. Rejected: decisions 0001/0003 already
   assume one long-lived stream for register, heartbeat, tasks, results, and
   advisories.
5. Include `group_by` bucket results in v1. Rejected: no accepted bucket
   schema yet; better to reject non-empty `group_by` than invent aggregates.

## Consequences

Positive:

- Proto implementation has an accepted, reviewable wire contract and schema v1
  semantics without inventing policy mid-code.
- Demo fixtures have an explicit compile mapping into `QueryTask`.
- Version skew across nodes (decision 0003) is handled by
  `query_schema_version` plus `UNSUPPORTED_VERSION`.

Tradeoffs:

- `group_by` is reserved on the task but unusable until a later schema bumps
  both task and result.
- Decimal-as-string and string field codes push semantic checks into
  validation layers 2–3 rather than protobuf decode alone.
- Control center and this node must share the same `.proto` and dictionary
  version; the control center repo does not exist yet, so this repository is
  the first home of the contract files when implemented.

## Follow-Up

- Implement the `.proto` files and codegen against this decision (no further
  wire-policy invention in that change).
- Add executable proof that the demo fixture compiles to a schema-v1
  `QueryTask` and that non-empty `group_by` is rejected.
- Design schema v2 (or a compatible extension) when grouped aggregates are
  required.
- Per-researcher / per-node dispatch authorization remains open from
  decision 0002 and is unchanged here.
