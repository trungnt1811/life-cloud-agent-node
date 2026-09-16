# Execution Plan: Implement The Life Cloud Node Agent

Date: 2026-09-16

## Status

Active

## Outcome

A production-ready node agent that: ingests a real hospital's own HIS/LIS
exports into a local canonical patient registry; dials out and holds a
long-lived gRPC stream to a control center; receives a `QueryTask`, runs it
through the full four-layer validation pipeline, executes it against the
local registry with resumable checkpointing, and returns a suppressed
aggregate `QueryResult`; exposes local REST admin for its own field
whitelist and update status; and is proven end-to-end against an in-repo
stub control center, since the real control center is a separate,
not-yet-built service.

## Context

- `docs/decisions/0001-node-agent-role-and-grpc-channel.md`: this repo is
  the per-hospital node agent; the **node dials out** and calls
  `NodeControl.Connect` as the gRPC **client** — the control center is the
  gRPC **server**, and is out of this repo's scope to implement for
  production. REST/Gin stays for node-local health/admin/Swagger.
- `docs/decisions/0002-federated-query-contract-and-execution.md`: async
  job model (control-center side), structured filter queries, four-layer
  node-side validation, two-layer field whitelisting (global schema vs.
  local `enabled_query_fields`), automatic execution (no per-query human
  approval), checkpointed resumable execution, aggregate-only suppressed
  output, explicit `QueryResult` status.
- `docs/decisions/0003-node-update-and-release-process.md`: control center
  only advises (`UpdateAdvisory` over the same stream); node logs/exposes
  it locally; applying an update is a `docker compose pull`/`up -d` human
  operation outside this codebase.
- `docs/decisions/0004-grpc-wire-contract-schema-v1.md` and
  `api/proto/lifecloud/node/v1/node_control.proto`: the wire contract is
  implemented (`gen/lifecloud/node/v1/`). `internal/adapters/federated/wire`
  already has a demo fixture compiler and
  `ValidateQueryTaskStructureV1` — **layer 1 (structural) and layer 4
  (version) are implemented and tested**; layers 2 (whitelist) and 3
  (semantic) are not.
- `docs/product/query-field-dictionary.md`: schema v1's global field set is
  9 numeric measurement codes (`HB`, `MCV`, `MCH`, `RBC`, `MCHC`, `RDW`,
  `HBA0`, `HBA2`, `HBF`), all eligible for all six comparison operators.
- `docs/product/federated-query-flow.md`: DFD 0/1/2 and the data flow
  dictionary this plan implements against; process IDs below (`3.1`–`3.6`,
  `4.0`, `5.0`) refer to that document.
- `life-cloud/data/federated_mvp/nodes/{VN_A,VN_B,VN_C}/`: the only
  concrete reference for real hospital export shapes and per-site
  `mapping.json` normalization rules (wide CSV, long CSV,
  semicolon-delimited UTF-8-BOM CSV). This plan's ingestion phase targets
  these three profiles; there is no other spec to build against.
- Current repo state: identity bootstrapped, decisions 0001-0004 recorded,
  proto contract implemented, structural+version validation implemented
  and tested. The domain layer is still the template's placeholder
  `example` CRUD — no patient/lab entities, no D3/D4/D5 stores, no gRPC
  server/client wiring, no ingestion, exist yet.

## Scope

In scope (this plan, phased — see Progress):

- Local Patient Registry (D3): patient/specimen/lab-observation domain
  model, repository, Postgres migration, covering the 9 schema-v1 fields.
- HIS/LIS ingestion (process 4.0) for the three life-cloud demo hospital
  profiles, normalizing into D3.
- Enabled Query Fields (D5) store + local REST admin (process 5.0).
- Query validation layers 2 (whitelist against D5) and 3 (semantic).
- Query execution (3.5) against D3 and output suppression (3.6).
- Job Progress Checkpoint (D4) and resumable execution.
- gRPC client: dial-out, `Register`/`Heartbeat`, receive `QueryTask`, run
  the pipeline, send `QueryResult`, reconnect/backoff.
- `UpdateAdvisory` handling (log + `GET /admin/status`).
- An in-repo, test-support-only stub `NodeControl` server to integration-
  test the client end-to-end, since no real control center exists.

Out of scope (explicitly not this plan):

- The control center itself (separate, not-yet-built repository).
- Removing or replacing the template's `example` CRUD code — unrelated
  placeholder, no reason to touch it while building a parallel domain.
- Per-researcher / per-node dispatch authorization (0002 Follow-Up),
  human-in-the-loop query approval (0002 rejected alternative), and
  high-severity advisory escalation (0003 Follow-Up) — named follow-ups in
  their decisions, not this plan.
- Hospital data-sharing agreements' actual content (which fields a real
  hospital enables) — this plan builds the `enabled_query_fields`
  mechanism, not any specific hospital's real configuration.

## Approach

Ordered by dependency; each phase is implemented, tested, and committed
before the next begins. Update this section and Decisions below as
evidence changes the approach.

1. **D3 — Local Patient Registry domain.** Entities/contracts for patient,
   specimen, and the 9 schema-v1 lab observations; repository port +
   GORM/Postgres adapter + migration, following the existing
   `internal/domain` / `internal/adapters/repositories` layering and
   architecture-boundary tests.
2. **Ingestion (4.0).** One adapter per life-cloud hospital profile
   (VN_A/B/C shapes), each implementing a shared `HospitalExportAdapter`
   port, writing normalized records into D3 via the repository from (1).
   Each profile's exact mapping rules get their own short decision note if
   materially ambiguous choices arise (e.g. an unmapped source code).
3. **D5 — Enabled Query Fields + admin REST.** Store + migration; REST
   endpoints under the existing Gin router (list/enable/disable a field,
   `updated_by`/`updated_at` audit), reusing the repo's existing
   admin-auth pattern (`SWAGGER_BASIC_AUTH_*`) rather than inventing a new
   one.
4. **Validation layers 2 & 3.** Whitelist against D5; semantic checks
   (operator eligible for field kind, value parses per
   `query-field-dictionary.md`, `time_range` bounds are valid dates) on
   top of the existing layer 1/4 in `internal/adapters/federated/wire`.
5. **Execution (3.5) + output policy (3.6).** Compile a validated
   `QueryTask` into a query against D3 (required panels + conditions +
   time range + specimen policy), compute `matching_count`, apply
   small-cell suppression before returning `QueryResult`.
6. **D4 — Job Progress Checkpoint + resume.** Store + migration; execution
   from (5) persists progress so a dropped stream resumes instead of
   rerunning. Checkpoint granularity is decided when this phase starts
   (needs its own small decision — see Risks).
7. **gRPC client.** Dial-out, `Register` on (re)connect with
   `agent_version`/`query_schema_version`, periodic `Heartbeat`, receive
   `CenterToNode` (`QueryTask` → pipeline from 4-6; `UpdateAdvisory` → log
   + expose via `GET /admin/status`), send `NodeToCenter`
   (`Register`/`Heartbeat`/`QueryResult`), reconnect with backoff on
   stream loss. Wired into `cmd/app` via `internal/di`.
8. **Stub control center (test support only).** A minimal `NodeControl`
   server — accept `Connect`, record `Register`/`Heartbeat`, send a
   `QueryTask` on command, receive `QueryResult` — living under test
   support (not shipped in the production binary), to drive end-to-end
   tests of (7) without the real control center.
9. **End-to-end integration test.** Stub control center + real node agent
   + seeded D3 (from 1-2): one query round trip, one rejected query
   (invalid field), one dropped-stream-and-resume scenario, one
   `UpdateAdvisory` delivery — proving the whole chain, not just units.

## Risks And Recovery

- Risk: ingestion mapping rules for VN_A/B/C have ambiguous or
  contradictory source values (life-cloud's own README flags this: max Hb
  41.1, max MCHC 316 in assumed units). Mitigation: preserve and report
  anomalies rather than silently correcting them, matching life-cloud's
  own stated policy; do not invent a cleaning rule without a decision.
- Risk: checkpoint granularity (phase 6) is underspecified today — no
  existing artifact defines what a resumable "batch" is for an aggregate
  query. Mitigation: stop at the start of phase 6 and record a short
  decision (batch definition, checkpoint write frequency) before
  implementing, per `docs/decisions/0002`'s stated follow-up need.
- Risk: this plan is large enough that context/priorities may drift across
  many sessions. Mitigation: keep Progress and Decisions current after
  every phase; a phase is not "done" without focused + integration proof
  per Validation.
- Recovery: each phase lands as its own commit on a clean `go build`/`go
  test ./...`/`make lint` baseline (established practice in this repo's
  history), so a bad phase can be reverted independently without unwinding
  earlier ones.

## Progress

- [ ] Phase 1 — D3 Local Patient Registry domain model + migration.
- [ ] Phase 2 — Ingestion adapters for VN_A/B/C profiles into D3.
- [ ] Phase 3 — D5 Enabled Query Fields store + admin REST.
- [ ] Phase 4 — Validation layers 2 (whitelist) & 3 (semantic).
- [ ] Phase 5 — Query execution (3.5) + output suppression (3.6).
- [ ] Phase 6 — D4 Job Progress Checkpoint + resumable execution.
- [ ] Phase 7 — gRPC client: dial-out, Register/Heartbeat, task handling.
- [ ] Phase 8 — In-repo stub control center (test support only).
- [ ] Phase 9 — End-to-end integration test across the full chain.

## Decisions

- 2026-09-16: MVP target is a **production-ready** agent (not a narrower
  contract-proof milestone) — user chose full scope over a reduced MVP.
- 2026-09-16: gRPC integration testing uses an **in-repo stub control
  center** (test-support only) rather than deferring integration testing
  until the real control center repository exists.
- 2026-09-16: **Real HIS/LIS ingestion is in scope**, targeting the three
  life-cloud demo hospital profiles (VN_A/B/C) as the concrete reference —
  the only available spec.
- Promote any phase-specific decision (e.g. checkpoint granularity,
  suppression threshold default, a specific mapping ambiguity) into
  `docs/decisions/` as that phase starts, per the pattern already used by
  0001-0004.

## Validation

- Focused proof: unit tests per phase (mapping tests, validator tests,
  repository tests using this repo's existing Postgres Testcontainers
  pattern).
- Integration or end-to-end proof: phase 9's stub-control-center-driven
  round trip, rejection, resume, and advisory scenarios.
- Repository-required checks per phase: `go build ./...`, `go vet ./...`,
  `go test ./...`, `make lint`, `make proto-check` (if the phase touches
  the contract), `make template-identity-check`.

## Result

Pending — phase 1 not yet started.
