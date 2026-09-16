# 0002 Federated Query Contract And Execution Model

Date: 2026-09-16

## Status

Accepted

## Context

Decision 0001 established that this node agent is the per-hospital side of a
federated query system, connected to a not-yet-built control center over a
node-initiated gRPC stream. It left open how a query actually travels from a
researcher to a node, how the node decides a query is safe to run, who runs
it, and what leaves the node afterward. This session's design discussion
worked through those questions end to end, grounded in the sibling
`life-cloud` repository's federated demo data (`data/federated_mvp/`,
`docs/superpowers/specs/2026-09-10-federated-demo-data-design.md`), which
already establishes structured `query_definition.json`-style filters, a
canonical field dictionary, and small-cell suppression as an open policy
item. This decision records the accepted answers.

## Decision

**Submission is asynchronous.** A client submits a cohort query to the
control center and immediately receives a `job_id`; it polls (or receives a
callback) for the result. No client ever talks to a node agent directly, and
no single slow or unreachable hospital blocks the request.

**Queries are structured filters, never raw SQL.** A query is a versioned,
schema-defined structure (field, operator, value, time range, group-by) —
the same shape as `life-cloud`'s `query_definition.json` fixtures — carried
in the `.proto` contract shared by node and control center. The control
center never has authority to run an arbitrary statement against a node's
database.

**Dispatch is to every online node, unfiltered, for now.** The control
center pushes a `QueryTask` to every node currently connected on its gRPC
stream (decision 0001). Per-researcher or per-node dispatch authorization is
explicitly out of scope for this decision (see Follow-Up); this is a stated
MVP assumption, not a production security posture.

**A node validates every `QueryTask` in four ordered layers before running
anything:**

1. Structural — the message decodes against the `.proto` contract.
2. Whitelist — every referenced field exists in *this node's own* local
   `enabled_query_fields` store.
3. Semantic — operators match field types, ranges and dates are
   well-formed.
4. Version — the task's `query_schema_version` is one this node's build
   understands.

Any failure returns a `QueryResult` carrying a specific rejection status
(`rejected_invalid_query` or `unsupported_version`) instead of running
anything. There is no silent partial execution of a partially-invalid query.

**Field whitelisting is two-layered, and the two layers have different
owners.** The shared `.proto` defines the global maximum set of fields and
operators the platform supports technically — Life Cloud owns this, and it
is identical for every node. Each node additionally keeps its own local
`enabled_query_fields` store (`field_code`, `enabled`, `updated_at`,
`updated_by`), which must be a subset of the global set and reflects that
specific hospital's actual data-sharing agreement with Life Cloud — not a
technical fact, an organizational one. A node validates layer 2 (whitelist)
against its own local store, never against the global set directly.

**Execution is fully automatic — no per-query human approval.** Once a
`QueryTask` passes all four validation layers, the node runs it immediately.
Safety is carried entirely by validation (above) and output shaping (below),
not by a manual gate. This was chosen deliberately over a human-in-the-loop
or hybrid policy-engine model (see Alternatives) to keep the query path fast
enough for interactive use at MVP stage.

**Execution is checkpointed locally.** A node persists its own progress for
a running task so that a dropped gRPC stream can resume from the last
checkpoint on reconnect (decision 0001's reconnect case) instead of
rerunning the task from scratch.

**Output is aggregate-only by construction and suppressed before it
leaves.** The result contract's type can only carry aggregate fields
(counts, sums, means) — there is no field capable of holding a
patient-level row, so no code path can leak one by mistake. Before a result
crosses the node boundary, small-cell suppression is applied against a
configured threshold (the policy `life-cloud`'s README flagged as "thuộc
node service sau này").

**Every result carries an explicit status, never a silent empty answer.**
`QueryResult.status` is one of `ok`, `rejected_invalid_query`,
`unsupported_version`, or `error`, with a human-readable `reason` when not
`ok`. The control center must be able to tell "no patients matched" apart
from "this node refused the query" or "this node failed."

**Query authoring needs a UI query-builder, owned by the control center.**
Researchers select fields/operators/values through a query-builder UI rather
than hand-writing JSON, to avoid field-name typos against the canonical
dictionary. That UI is the control center's responsibility and is out of
scope for this repository.

## Alternatives Considered

1. Raw SQL sent to the node. Rejected: unacceptable injection and trust
   surface against a hospital's own database.
2. Synchronous client request/response. Rejected: hospital network
   reliability is not good enough to hold a client connection open across a
   federated round trip; one slow site would block every query.
3. Human-in-the-loop approval per query, and a hybrid auto/manual policy
   engine. Both rejected for now: per-query manual approval breaks the
   async, interactive query model this MVP targets, and the hybrid
   policy-engine model adds a local approval queue and UI this stage
   doesn't need yet. Revisit if a hospital's governance later requires it.
4. One global whitelist shared identically by every node, with no per-node
   override. Rejected: it doesn't reflect that each hospital's data-sharing
   agreement with Life Cloud can differ.

## Consequences

Positive:

- A strong technical safety net (four-layer validation + output
  suppression) despite no manual approval step, keeping the query path fast.
- Clear separation of ownership: Life Cloud owns the global schema; each
  hospital owns its own enabled-fields subset and audit trail.
- Checkpointed execution tolerates the flaky hospital networking already
  assumed in decision 0001.
- Explicit status codes make federated query failures debuggable instead of
  looking like "no data."

Tradeoffs:

- No manual override or audit gate before execution — correctness and
  safety depend entirely on the validation rules being right, since nothing
  else stops a technically-valid-but-unwise query from running.
- The local `enabled_query_fields` store is a new node-local entity and
  admin surface (with audit fields) that has to be built and kept current.
- Dispatch-to-all-online-nodes with no researcher/node authorization is
  explicitly not acceptable once real patient data and hospital consent
  requirements are in play — it must be revisited before that happens.

## Follow-Up

- Design per-researcher / per-node dispatch authorization before any real
  patient data flows through this system; today's "dispatch to all online
  nodes" is a named MVP assumption, not a security decision.
- Concrete `.proto` / schema v1 wire contract: addressed by decision 0004
  and `docs/product/query-field-dictionary.md` (implementation of `.proto`
  files still pending).
- Design the local `enabled_query_fields` admin surface (REST endpoint(s),
  audit fields) as node-local REST scope per decision 0001.
- Revisit the human-in-the-loop / hybrid policy-engine alternative if a
  hospital's governance requirements exceed what automatic validation can
  guarantee.
