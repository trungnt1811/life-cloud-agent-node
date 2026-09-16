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
evidence changes the approach. File paths are proposals, not authority —
follow the repo's existing Clean Architecture layering
(`internal/domain` never imports `gen/`, `internal/adapters` implements
domain ports, `internal/delivery` stays HTTP-only) and its
`internal/testsupport/archtest` guards over anything written here.

### Phase 1 — D3: Local Patient Registry domain model + migration

Files: `internal/domain/entities/{patient,specimen,lab_observation}.go`,
`internal/domain/repositories/interfaces.go` (add `PatientRegistryRepository`),
`internal/adapters/repositories/models/{patient,specimen,lab_observation}.go`,
`internal/adapters/repositories/patient_registry_repository.go` (+ mapper
+ test), `internal/adapters/postgres/scripts/02_create_patient_registry_tables.sql`,
`internal/mocks/mock_patient_registry_repository.go`.

Key types (storage-only this phase; query methods land in Phase 5):

```go
type Patient struct {
    ID                 uuid.UUID
    ExternalPatientID  string // source-system identifier, kept as-is
}

type Specimen struct {
    ID              uuid.UUID
    PatientID       uuid.UUID
    CollectedAt     time.Time // calendar date only
    SourceDataset   string
    SourceFile      string
    SourceRowNumber int
    SourceRecordID  string
}

type LabObservation struct {
    ID         uuid.UUID
    SpecimenID uuid.UUID
    FieldCode  string // one of the 9 codes in query-field-dictionary.md
    Value      string // decimal string, exact precision preserved
    Censored   bool   // true when source used a '<' operator
    RawValue   string
    RawUnit    string
}
```

- [x] Entities carry no GORM tags (architecture boundary).
- [x] Migration creates `patients`, `specimens`, `lab_observations` with a
      FK chain and indexes on `(patient_id, collected_at)` and
      `(specimen_id, field_code)`.
- [x] GORM models + mapper, following `example_mapper.go`'s pattern.
- [x] Repository implementation + Postgres Testcontainers test (reuse
      `internal/adapters/repositories/testsupport`).
- [x] `make mockgen`; `go build ./...`; `go test ./...`; `make lint`;
      `make template-identity-check`.

### Phase 2 — Ingestion (4.0): VN_A/B/C adapters into D3

Files: `internal/adapters/ingestion/{adapter.go,vn_a.go,vn_b.go,vn_c.go}`
(+ one `_test.go` each), `internal/domain/usecases/ingest_hospital_export_ucase.go`,
a `cmd/` entrypoint or `make ingest` target, small fixtures under
`internal/adapters/ingestion/testdata/` copied from
`life-cloud/data/federated_mvp/nodes/{VN_A,VN_B,VN_C}/raw/` (a handful of
rows each, not the full dataset).

Key types:

```go
type Adapter interface {
    ParseFile(path string) ([]NormalizedSpecimen, []Anomaly, error)
}

type NormalizedSpecimen struct {
    ExternalPatientID string
    CollectedAt       time.Time
    Observations      []NormalizedObservation
    Provenance        Provenance // source file/row/record id
}

type NormalizedObservation struct {
    FieldCode string
    Value     string // decimal, unrounded
    Censored  bool
    RawValue  string
    RawUnit   string
}

// Anomaly is a row that couldn't be normalized; recorded, not silently
// dropped or corrected (matches life-cloud's own stated policy).
type Anomaly struct {
    SourceFile      string
    SourceRowNumber int
    Reason          string
}
```

- [ ] VN_A adapter: wide CSV, one row per panel, ISO dates, Hb in `g/dL`
      directly. Unit test against a small fixture.
- [ ] VN_B adapter: long CSV, one row per test, site codes `0301`/`H02`,
      Hb `g/L` → `g/dL` conversion, HbA2 fraction → percent, `YYYYMMDD`
      dates. Unit test.
- [ ] VN_C adapter: semicolon-delimited UTF-8-BOM CSV, unaccented
      Vietnamese headers, comma decimal separator, `DD/MM/YYYY` dates.
      Unit test.
- [ ] Anomalies (unknown codes, missing units, ambiguous dates, negative
      values) are recorded via the `Anomaly` return, never silently
      corrected or dropped. If a genuinely ambiguous mapping choice comes
      up implementing any of the three, stop and record it as a short
      decision before choosing (per Risks).
- [ ] Ingestion usecase: pick adapter by profile flag, call
      `PatientRegistryRepository` from Phase 1 to persist normalized
      output; log anomaly count/reasons.
- [ ] Integration test: ingest the copied fixtures end-to-end, assert
      expected patient/specimen/observation counts land in D3.

### Phase 3 — D5: Enabled Query Fields store + admin REST

Files: `internal/domain/entities/enabled_query_field.go`,
`internal/domain/repositories/interfaces.go` (add
`EnabledQueryFieldRepository`), matching adapter/model/mapper/test,
`internal/adapters/postgres/scripts/03_create_enabled_query_fields_table.sql`,
`internal/domain/usecases/enabled_query_field_ucase.go`,
`internal/delivery/dto/enabled_query_field.go`,
`internal/delivery/http/handlers/enabled_query_field_handler.go`,
`internal/delivery/http/route/enabled_query_field_route.go`.

```go
type EnabledQueryField struct {
    FieldCode string
    Enabled   bool
    UpdatedAt time.Time
    UpdatedBy string
}
```

Endpoints (admin-auth protected, same middleware family as Swagger's):
`GET /admin/query-fields` (list all 9 global codes + this node's state),
`PUT /admin/query-fields/:field_code` (`{enabled, updated_by}`).

- [ ] Migration seeds all 9 global field codes with `enabled=false` —
      explicit per-hospital opt-in, not a default-open list.
- [ ] Repository + usecase + handler + route, following the `example_*`
      layering exactly.
- [ ] Reuse `SWAGGER_BASIC_AUTH_USER`/`_PASS`-style middleware for these
      admin routes; if that coupling turns out wrong once written, flag it
      rather than silently introducing a second auth scheme.
- [ ] `make swagger` updated; handler + repository tests; `go build/test/lint`.

### Phase 4 — Validation layers 2 (whitelist) & 3 (semantic)

Files: new files alongside `internal/adapters/federated/wire/compile_demo_fixture.go`,
e.g. `validate_whitelist.go`, `validate_semantic.go`, `field_dictionary.go`
(the 9-code → kind map, comment-linked to
`docs/product/query-field-dictionary.md`), plus a combined
`ValidateQueryTaskV1` that runs all four layers in Fig. 3's order.

```go
// ValidateQueryTaskWhitelistV1 — layer 2: every field_code referenced must
// exist and be enabled in this node's own EnabledQueryFieldRepository.
func ValidateQueryTaskWhitelistV1(ctx context.Context, task *nodev1.QueryTask, repo EnabledQueryFieldRepository) error

// ValidateQueryTaskSemanticV1 — layer 3: operator eligible for field kind,
// value parses as an exact decimal, time_range bounds are valid and ordered.
func ValidateQueryTaskSemanticV1(task *nodev1.QueryTask) error
```

- [ ] Whitelist: unknown or disabled `field_code` in any condition or
      required panel → `REJECTED_INVALID_QUERY`, reason names the field.
- [ ] Semantic: value parses as an unrounded decimal (reuse the pattern
      `life-cloud`'s own generator uses — no binary float on the wire, per
      the `.proto` comment on `ConditionValue`); `time_range.from <=
      time_range.to`; both are valid `YYYY-MM-DD`.
- [ ] Combine all four layers into one `ValidateQueryTaskV1` matching
      Fig. 3's exact order (structural → whitelist → semantic → version),
      used by the gRPC handler in Phase 7 instead of calling each layer
      separately.
- [ ] One test per rejection reason, following the existing
      `TestQueryTaskV1_RejectsStructuralViolations` pattern.

### Phase 5 — Execution (3.5) + output policy (3.6)

Files: extend `PatientRegistryRepository` with `CountMatchingCohort`;
implement the SQL in the Phase 1 repository; add
`internal/adapters/federated/wire/execute_query.go`; add
`SUPPRESSION_THRESHOLD` to `conf`/`.env.example` (default e.g. `5`).

```go
// CohortCriteria is domain-owned, decoupled from the wire QueryTask.
type CohortCriteria struct {
    From, To        time.Time
    Conditions      []Condition
    RequiredPanels  []RequiredPanel
}

func (r *patientRegistryRepository) CountMatchingCohort(ctx context.Context, c CohortCriteria) (uint64, error)
```

- [ ] SQL selects the latest specimen per patient within `[from, to]`
      (per `SPECIMEN_POLICY_LATEST_IN_RANGE`), requires every
      `RequiredPanel`'s fields present (EXACT = non-censored value
      required; ALLOW_CENSORED = presence is enough even if censored),
      then applies each `Condition` as a decimal comparison.
- [ ] Suppression: `0 < count < SUPPRESSION_THRESHOLD` → `matching_count=0,
      suppressed=true`; otherwise the real count, `suppressed=false`.
- [ ] Postgres Testcontainers test seeding known fixture rows with a
      hand-computed expected count (mirrors life-cloud's own benchmark
      verification style, not just a smoke test).

### Phase 6 — D4: Job Progress Checkpoint + resume

- [ ] **Before writing code:** record
      `docs/decisions/0005-job-checkpoint-granularity.md` — what a
      resumable batch is (e.g. patient-ID range chunks) and how often a
      checkpoint is written. Do not guess this in code first.
- [ ] Entity + migration for `job_progress` (`job_id`, `last_checkpoint`,
      `updated_at`).
- [ ] Repository + resume logic wired into Phase 5's execution so it runs
      in checkpointable chunks.
- [ ] Test: interrupt execution mid-chunk, resume, assert the final count
      matches running the same query uninterrupted (no missed or
      double-counted chunk).

### Phase 7 — gRPC client: dial-out, Register/Heartbeat, task handling

Files: `internal/adapters/federated/client/node_client.go`,
`internal/workers/federated_client_worker.go`, `internal/runtimeconfig`
additions (`CONTROL_CENTER_ADDRESS`, `NODE_ID`, `AGENT_VERSION`), DI wiring
in `internal/di/wire.go` and `cmd/app/app.go`.

- [ ] Dial-out and open `NodeControl.Connect`'s bidi stream.
- [ ] Send `Register` on connect and every reconnect
      (`node_id`, `agent_version`, `query_schema_version=1`).
- [ ] Heartbeat every 30s with in-flight job IDs (Fig. 4).
- [ ] Receive loop: `QueryTask` → `ValidateQueryTaskV1` (Phase 4) →
      execute (Phase 5, checkpointed per Phase 6) → send `QueryResult`;
      `UpdateAdvisory` → persist/expose for `GET /admin/status` (decision
      0003 — log only, never auto-apply).
- [ ] Reconnect with exponential backoff on stream error/EOF; in-flight
      job resumes from its Phase 6 checkpoint after reconnect.
- [ ] Graceful shutdown via the existing `internal/server/shutdown.go`
      lifecycle hook.
- [ ] Unit tests for the client's message handling using an in-process
      `bufconn` server (doesn't need Phase 8's stub — a minimal inline
      test double is enough here).

### Phase 8 — Stub control center (test support only)

Files: `internal/testsupport/controlcenterstub/server.go` implementing
`nodev1.NodeControlServer`.

- [ ] Accept `Connect`, record `Register`/`Heartbeat`, expose
      `SendQueryTask(task)` / `SendUpdateAdvisory(...)` plus channels to
      observe received `QueryResult`s — enough surface to drive Phase 9,
      nothing more.
- [ ] Serves over `bufconn` or `127.0.0.1:0`, test-only.
- [ ] Package doc comment states plainly this is test support and must
      never be imported from `cmd/`; extend
      `internal/testsupport/archtest` so that import is a caught
      violation, not just a convention.

### Phase 9 — End-to-end integration test

Files: `tests/integration/federated_query_test.go`, added to CI.

- [ ] Boot the Phase 7 client against the Phase 8 stub; seed D3 (Phase
      1-2 fixtures); enable the needed fields in D5 (Phase 3); send a
      valid `QueryTask`; assert `QueryResult{status: OK, matching_count:
      <expected>}`.
- [ ] Rejected case: reference a disabled/unknown field → assert
      `REJECTED_INVALID_QUERY`.
- [ ] Dropped-stream case: sever the stub connection mid-task, let the
      client reconnect, assert the result still arrives via Phase 6's
      checkpoint resume.
- [ ] `UpdateAdvisory` case: stub sends an advisory, assert
      `GET /admin/status` reflects it.
- [ ] Wire into `.github/workflows/ci.yml` (new `make test-integration`
      target or folded into the existing Postgres-repository test job).

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

- [x] Phase 1 — D3 Local Patient Registry domain model + migration.
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
- 2026-09-16 (Phase 1): Domain entities follow the existing encapsulated
  `ExampleEntity` pattern (private fields + `Record()`), not exported
  structs from the plan sketch. Storage API is
  `GetOrCreatePatient` + transactional `SaveSpecimen` (with observations);
  cohort query methods remain Phase 5. Unique `(specimen_id, field_code)`
  enforces one observation per field per specimen.
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

Phase 1 complete — D3 local patient registry domain, migration, GORM
repository, mocks, and Postgres Testcontainers proof are in place. Phase 2
(ingestion) is next.
