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

- [x] VN_A adapter: wide CSV, one row per panel, ISO dates, Hb in `g/dL`
      directly. Unit test against a small fixture.
- [x] VN_B adapter: long CSV, one row per test, site codes `0301`/`H02`,
      Hb `g/L` → `g/dL` conversion, HbA2 fraction → percent, `YYYYMMDD`
      dates. Unit test.
- [x] VN_C adapter: semicolon-delimited UTF-8-BOM CSV, unaccented
      Vietnamese headers, comma decimal separator, `DD/MM/YYYY` dates.
      Unit test.
- [x] Anomalies (unknown codes, missing units, ambiguous dates, negative
      values) are recorded via the `Anomaly` return, never silently
      corrected or dropped. If a genuinely ambiguous mapping choice comes
      up implementing any of the three, stop and record it as a short
      decision before choosing (per Risks).
- [x] Ingestion usecase: pick adapter by profile flag, call
      `PatientRegistryRepository` from Phase 1 to persist normalized
      output; log anomaly count/reasons.
- [x] Integration test: ingest the copied fixtures end-to-end, assert
      expected patient/specimen/observation counts land in D3.

### Phase 3 — D5: Enabled Query Fields store + admin REST

Files: `internal/domain/entities/enabled_query_field.go`,
`internal/domain/repositories/enabled_query_field.go` (add
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

- [x] Migration seeds all 9 global field codes with `enabled=false` —
      explicit per-hospital opt-in, not a default-open list.
- [x] Repository + usecase + handler + route, following the `example_*`
      layering exactly.
- [x] Reuse `SWAGGER_BASIC_AUTH_USER`/`_PASS`-style middleware for these
      admin routes; if that coupling turns out wrong once written, flag it
      rather than silently introducing a second auth scheme.
- [x] `make swagger` updated; handler + repository tests; `go build/test/lint`.

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

- [x] Whitelist: unknown or disabled `field_code` in any condition or
      required panel → `REJECTED_INVALID_QUERY`, reason names the field.
- [x] Semantic: value parses as an unrounded decimal (reuse the pattern
      `life-cloud`'s own generator uses — no binary float on the wire, per
      the `.proto` comment on `ConditionValue`); `time_range.from <=
      time_range.to`; both are valid `YYYY-MM-DD`.
- [x] Combine all four layers into one `ValidateQueryTaskV1` matching
      Fig. 3's order (version runs first, then structural → whitelist →
      semantic — see the Phase 4 post-review Decisions entry),
      used by the gRPC handler in Phase 7 instead of calling each layer
      separately.
- [x] One test per rejection reason, following the existing
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

- [x] SQL selects the latest specimen per patient within `[from, to]`
      (per `SPECIMEN_POLICY_LATEST_IN_RANGE`), requires every
      `RequiredPanel`'s fields present (EXACT = non-censored value
      required; ALLOW_CENSORED = presence is enough even if censored),
      then applies each `Condition` as a decimal comparison.
- [x] Suppression: `0 < count < SUPPRESSION_THRESHOLD` → `matching_count=0,
      suppressed=true`; otherwise the real count, `suppressed=false`.
- [x] Postgres Testcontainers test seeding known fixture rows with a
      hand-computed expected count (mirrors life-cloud's own benchmark
      verification style, not just a smoke test).

### Phase 6 — D4: Job Progress Checkpoint + resume

- [x] **Before writing code:** record
      `docs/decisions/0005-job-checkpoint-granularity.md` — what a
      resumable batch is (e.g. patient-ID range chunks) and how often a
      checkpoint is written. Do not guess this in code first.
- [x] Entity + migration for `job_progress` (`job_id`, `last_checkpoint`,
      `updated_at`).
- [x] Repository + resume logic wired into Phase 5's execution so it runs
      in checkpointable chunks.
- [x] Test: interrupt execution mid-chunk, resume, assert the final count
      matches running the same query uninterrupted (no missed or
      double-counted chunk).

### Phase 7 — gRPC client: dial-out, Register/Heartbeat, task handling

Files: `internal/adapters/federated/client/node_client.go`,
`internal/workers/federated_client_worker.go`, `internal/runtimeconfig`
additions (`CONTROL_CENTER_ADDRESS`, `NODE_ID`, `AGENT_VERSION`), DI wiring
in `internal/di/wire.go` and `cmd/app/app.go`.

- [x] **Before writing code:** record
      `docs/decisions/0006-grpc-client-transport-security.md` — TLS was not
      decided by 0001/0004; required a decision rather than a default
      picked while writing the client.
- [x] Dial-out and open `NodeControl.Connect`'s bidi stream.
- [x] Send `Register` on connect and every reconnect
      (`node_id`, `agent_version`, `query_schema_version=1`).
- [x] Heartbeat every 30s with in-flight job IDs (Fig. 4).
- [x] Receive loop: `QueryTask` → `ValidateQueryTaskV1` (Phase 4) →
      execute (Phase 5, checkpointed per Phase 6) → send `QueryResult`;
      `UpdateAdvisory` → persist/expose for `GET /admin/status` (decision
      0003 — log only, never auto-apply).
- [x] Reconnect with exponential backoff on stream error/EOF; in-flight
      job resumes from its Phase 6 checkpoint after reconnect.
- [x] Graceful shutdown via the existing `internal/server/shutdown.go`
      lifecycle hook.
- [x] Unit tests for the client's message handling using an in-process
      `bufconn` server (doesn't need Phase 8's stub — a minimal inline
      test double is enough here).

### Phase 8 — Stub control center (test support only)

Files: `internal/testsupport/controlcenterstub/server.go` implementing
`nodev1.NodeControlServer`.

- [x] Accept `Connect`, record `Register`/`Heartbeat`, expose
      `SendQueryTask(task)` / `SendUpdateAdvisory(...)` plus channels to
      observe received `QueryResult`s — enough surface to drive Phase 9,
      nothing more.
- [x] Serves over `bufconn` or `127.0.0.1:0`, test-only.
- [x] Package doc comment states plainly this is test support and must
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
- [x] Phase 2 — Ingestion adapters for VN_A/B/C profiles into D3.
- [x] Phase 3 — D5 Enabled Query Fields store + admin REST.
- [x] Phase 4 — Validation layers 2 (whitelist) & 3 (semantic).
- [x] Phase 5 — Query execution (3.5) + output suppression (3.6).
- [x] Phase 6 — D4 Job Progress Checkpoint + resumable execution.
- [x] Phase 7 — gRPC client: dial-out, Register/Heartbeat, task handling.
- [x] Phase 8 — In-repo stub control center (test support only).
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
- 2026-09-16 (Phase 1, post-review fixes): `/code-review` on the Phase 1
  commit found and this session fixed: a date-shift bug in
  `calendarDateUTC` for positive-offset timezones (Vietnam, UTC+7 — fixed
  by not converting to UTC before extracting the calendar date);
  `NewSpecimen`/`NewLabObservation` took several same-typed positional
  string args, now grouped into `NewSpecimenParams`/
  `NewLabObservationParams`; `SaveSpecimen` had no idempotency guard,
  now uses `ON CONFLICT DO NOTHING` on a new `specimens` unique key
  (`source_dataset, source_file, source_record_id`) plus a canonical
  re-read, and batches observation inserts instead of looping; added
  `specimens_source_provenance_key`; removed a redundant plain index that
  duplicated the `lab_observations` unique constraint's backing index;
  added `patientRegistryRepository.WithTx` for transaction composability;
  reverted `ExampleRepository`'s mockgen directive to `-source` mode and
  moved `PatientRegistryRepository` into its own file with its own
  `-source` directive (a `reflect`-mode directive had made mock
  regeneration for either interface depend on the whole package
  compiling); factored duplicated `dbWithContext` into
  `repohelpers.DBWithContext`; `GetOrCreatePatient` now upserts with
  `DoUpdates` (a no-op self-assignment) so Postgres always `RETURNING`s
  the canonical row in one round trip instead of up to three.
- 2026-09-17 (Phase 2): Adapter interface + normalized types live in
  domain (`types` + usecase port); CSV parsers stay in
  `internal/adapters/ingestion`. Profile selection is by usecase map keyed
  `VN_A`/`VN_B`/`VN_C`. Unit conversion uses exact `big.Rat` factors from
  each site's `mapping.json` (`g/L→g/dL` ×0.1, HPLC fraction→% ×100).
  Ambiguous/blank date formats quarantine the whole specimen/row as
  `ambiguous_date` (no silent DMY/MDY choice).
- 2026-09-17 (Phase 1/2 boundary, messy-data revision handling, first
  pass — **superseded by the entry below**): a re-ingested `(specimen_id,
  field_code)` overwrote `value`/`censored`/`raw_value`/`raw_unit`
  unconditionally (`ON CONFLICT DO UPDATE`) instead of `DO NOTHING`, and
  the specimen row still deduped on file/row provenance
  (`source_dataset, source_file, source_record_id`). `/code-review` on
  the Phase 2 commit found this was last-write-wins, not
  highest-revision-wins, and that file/row provenance is the wrong
  specimen identity (a genuinely corrected re-export under a new
  filename would create a duplicate specimen, not update the existing
  one) — see the follow-up fix below.
- 2026-09-17 (Phase 1/2 boundary, messy-data revision handling — current):
  fixed the three correctness gaps `/code-review` found in the ingestion
  pipeline (`docs/product/federated-query-flow.md` Fig. 6's dashed-amber
  nodes):
  - **Specimen identity.** Added `external_specimen_id` (the hospital's own
    SpecimenNo/sample_id/MaMau, threaded through both `wide.go` and
    `long.go`, previously parsed and discarded). `specimens`'s unique key
    changed from `(source_dataset, source_file, source_record_id)` to
    `(source_dataset, patient_id, external_specimen_id)` — a corrected
    re-export under a new filename now correctly updates the existing
    specimen instead of duplicating it. `wide.go` now validates it
    non-empty (`missing_specimen_id`), matching `long.go` — closing the
    blank-`source_record_id`-merges-two-specimens gap for free, since
    file/row provenance is no longer the identity key at all.
  - **Revision, not call order.** Added a `revision` column to
    `lab_observations`. The upsert's `ON CONFLICT ... DO UPDATE` now
    carries a `WHERE lab_observations.revision <= EXCLUDED.revision`
    guard (verified against real Postgres, not just assumed from the
    GORM API) — a stale re-ingest of an older export can no longer
    clobber an already-corrected result, regardless of which ingest run
    executes last.
  - **`long.go` grouping gaps.** A group whose rows disagree on collection
    date now quarantines the whole specimen (`date_mismatch_in_group`)
    instead of silently keeping only the first row's date. Two rows at
    the same revision for the same test code with different values are
    now excluded and flagged (`conflicting_value_same_revision:<code>`)
    instead of one silently winning by file order.
  Covered by `TestPatientRegistryRepository_SaveSpecimenIsIdempotentOnRetry`,
  `TestPatientRegistryRepository_SaveSpecimenHigherRevisionWinsRegardlessOfCallOrder`
  (both directions of call order), `TestVNAAdapter_FlagsMissingSpecimenID`,
  `TestVNBAdapter_FlagsDateMismatchWithinGroup`, and
  `TestVNBAdapter_FlagsConflictingValueAtSameRevision`.
- 2026-09-17 (completeness pass — remaining simplification/efficiency
  findings from the same review): fixed the ones with real, contained
  value; left the ones whose fix cost or risk outweighed it.
  - **Fixed**: N+1 patient upsert (`ingestHospitalExportUseCase` now caches
    `*entities.Patient` by external ID for one `IngestFiles` run instead of
    re-upserting per specimen); `SaveSpecimen`'s unconditional read-back
    SELECT after the specimen insert (now only reads back when
    `RowsAffected == 0`, i.e. an actual conflict — the common first-time
    path is one round trip, matching the pattern already used in
    `GetOrCreatePatient`); the redundant `IngestResult.Anomalies` counter
    (removed; `AnomalyCount()` derives it from `len(AnomalyReasons)`, so
    the two can't drift); dead code — unused `AdapterForProfile`, the
    unused `name` field on both profile structs, and the unused `status`
    field on `longRow`; four copy-pasted "default ID / default timestamp"
    blocks across `Patient`/`Specimen`/`LabObservation` constructors,
    factored into `entities.ensureID`/`ensureTimestamp` (the pre-existing,
    out-of-scope `example.go` entity was left alone); the identical
    patient/specimen/status/revision preamble duplicated between `wide.go`
    and `long.go`, factored into `normalize.go`'s `validateRowHeader`.
  - **Left open**: the hardcoded case-sensitive `"final"` status literal;
    the dual date-format representation (`expectedDateFormat` strptime
    token vs. `dateLayout` Go layout) that can drift out of sync for a
    future profile; the `wideProfile`/`longProfile` shared-field
    duplication; the per-entity mapper boilerplate
    (`patient_registry_mapper.go` vs. `example_mapper.go`); `cmd/ingest`
    duplicating `cmd/migration`'s config/DB bootstrap sequence. None of
    these affect correctness; revisit if/when a real 4th/5th hospital
    profile is added, which is when the date-format and shared-field
    duplication risk actually bites.
- 2026-09-18 (Phase 3): D5 enabled-query-fields store + admin REST.
  Entity follows the private-fields + `Record()` pattern. Repository
  lives in its own file with `-source` mockgen (same as patient
  registry). `ListQueryFields` always returns all 9 schema-v1 codes
  from `internal/domain/queryfields`, merging stored rows so a truncated
  or partially seeded DB still presents the full dictionary; synthetic
  rows omit `updated_at`. `PUT` rejects unknown codes, requires
  `updated_by`, and requires explicit `enabled` (`*bool`, no silent
  disable on omit). Admin routes reuse `SWAGGER_BASIC_AUTH_USER`/`_PASS`
  via shared `HTTPBasicAuth` (realm `admin`) and are **never** registered
  without both credentials (stricter than Swagger's empty-open non-prod
  behavior). Migration seeds all 9 codes `enabled=false`; seed↔Go list
  drift is guarded by `queryfields.TestMigrationSeedMatchesSchemaV1FieldCodes`.
- 2026-09-18 (Phase 3 post-review fixes): `/code-review` on the D5 admin
  REST commits found 10 issues; fixed the ones with real correctness or
  security value.
  - **Fixed**: admin routes now gate on their own `ADMIN_BASIC_AUTH_USER/PASS`
    instead of reusing `SWAGGER_BASIC_AUTH_USER/PASS` — rotating docs
    access no longer also rotates the ability to change which clinical
    fields this node exposes. Registration is now a 3-way switch: both set
    → register; both blank → warn and skip (intentional non-prod state);
    exactly one set → `log.Panic`/`panic` at startup (was: silently
    register nothing and 404 forever). `NewEnabledQueryFieldFromRecord` now
    normalizes `field_code` via `queryfields.NormalizeFieldCode`, matching
    `NewEnabledQueryField` (previously only `NewEnabledQueryField`
    normalized, so a hand-run SQL row with a lowercase code would silently
    miss `GetByFieldCode`/`ListQueryFields` lookups). `updated_by` is now
    derived from the authenticated Basic Auth identity
    (`middleware.AuthenticatedUserContextKey`, set by `HTTPBasicAuth`) —
    the request DTO no longer accepts an `updated_by` field a client could
    set to any value. `UpdateQueryField` goes straight to the atomic
    `Upsert` (`ON CONFLICT ... DO UPDATE`); removed the preceding
    `GetByFieldCode` call, which was both wasted (nothing from the fetched
    row was used) and a check-then-act race between concurrent `PUT`s for
    the same `field_code`. `swagger_basic_auth.go`/`_test.go` renamed to
    `basic_auth.go`/`_test.go` (the middleware is shared infra, not
    Swagger-specific — it now also gates `/admin/query-fields`). Repository
    `Upsert` now returns an error for a nil entity or blank `field_code`
    instead of silently no-oping (documented on the interface); dropped
    the dead `WithTx` method (never on the interface, never called, no
    `TransactionManager` wiring) and the nil-pointer guards in the two
    single-item mapper functions, which were unreachable at every current
    call site. Added `enabled_query_field_mapper_test.go`
    (`mappingtest.AssertSameNamedFieldCoverage` + round-trip test), closing
    the one place D5 diverged from the patient-registry mapper test
    pattern.
  - **Left open**: none — all 10 findings were either fixed above or folded
    into a fix above (e.g. the partial-credential 404 and the shared-secret
    findings both resolved by the credential split).
- 2026-09-18 (Phase 4 — layer order and in-place normalization
  superseded by the post-review entry below): Four-layer `ValidateQueryTaskV1` (Fig. 3 order:
  structural → whitelist → semantic → version). Schema-version check moved
  out of `ValidateQueryTaskStructureV1` into `ValidateQueryTaskVersionV1`
  so unsupported versions map to `UNSUPPORTED_VERSION` only after earlier
  layers pass. Failures use typed `QueryValidationError` with the
  corresponding `QueryResultStatus`. Whitelist reads D5 via
  `EnabledQueryFieldRepository.ListAll`; semantic requires exact decimal
  `number_value` (`big.Rat`, no `e`/`/`), measurement ops only with
  `number_value`, and ordered `YYYY-MM-DD` time bounds. Field kinds are
  derived from `queryfields.SchemaV1FieldCodes` (no second hard-coded
  list). Structural validation trims/normalizes time bounds and
  `field_code`s in place. Nil repo / D5 `ListAll` failures use
  `QUERY_RESULT_STATUS_ERROR`, not `REJECTED_INVALID_QUERY`.
- 2026-09-18 (Phase 4 post-review fixes): `/code-review` on the Phase 4
  commits found 6 issues; all fixed.
  - **Version gate first.** `ValidateQueryTaskV1` now runs version →
    structural → whitelist → semantic (Fig. 3 and ADR 0002 updated). Layers
    1-3 hard-code v1 rules, so a v2 task was answered
    `REJECTED_INVALID_QUERY`, cost a D5 read, and turned into `ERROR` on a
    D5 outage. Supersedes the "structural → … → version" order above.
  - **Strict decimals.** `number_value` must match `^-?\d+(\.\d+)?$` and be
    ≤ 32 chars. `big.Rat.SetString` alone accepted `0x10`, `0b101`,
    `1_000`, `0x1p4`, `+5`, `-.5` and unbounded-length input (~750ms CPU
    for a 1M-digit hex string).
  - **No mutation, no trimming.** `ValidateQueryTaskStructureV1` no longer
    rewrites the task (previously trimmed `time_range`, rewrote field
    codes, and could leave it half-normalized on failure). Padded dates and
    decimals are now *rejected* by layer 3 instead of trimmed. Field codes
    may still arrive padded/lower-case: every layer normalizes on read, and
    the Phase 5 executor must too (`queryfields.NormalizeFieldCode`).
  - **Fail-closed whitelist.** Rows that collide after normalization enable
    a code only if all of them are enabled, so row order can't turn a
    disabled field on.
- 2026-09-18 (Phase 5): `CountMatchingCohort` on D3 selects
  `LATEST_IN_RANGE` via `ROW_NUMBER() … ORDER BY collected_at DESC,
  external_specimen_id DESC` (life-cloud benchmark / decision 0004), then
  applies required panels (`EXACT` ⇒ present + `censored=false`;
  `ALLOW_CENSORED` ⇒ present) and measurement conditions as
  `value::numeric` comparisons. Domain criteria live in
  `internal/domain/types/cohort.go` (no `gen/` import). Wire
  `ExecuteQueryTaskV1` maps a validated task → criteria → count →
  process 3.6 suppression. `SUPPRESSION_THRESHOLD` defaults to `5`
  (`conf` / `.env.example`); `0 < raw < threshold` yields
  `matching_count=0, suppressed=true`. Proof:
  `TestPatientRegistryRepository_CountMatchingCohort_DemoRuleFixture`
  (hand-computed expected count 2) plus wire suppression/mapping tests.
- 2026-09-19 (Phase 5 post-review fixes): `/code-review` on the Phase 5
  commit found 7 issues; all fixed.
  - **Suppression fails closed.** Threshold `0` now means "use the default
    5" (`constants.DefaultSuppressionThreshold`, shared with `conf`), never
    "disabled" — a mis-wired Phase 7 caller can no longer leak raw counts
    of 1-4. A threshold of `1` hides nothing (tests).
  - **Conditions ignore censored observations.** `'<0.1'` is stored as
    `0.1, censored=true`; that is a bound, not a measurement, so
    `conditionSQL` adds `censored = FALSE`. Required panels keep their own
    `EXACT` / `ALLOW_CENSORED` handling. Chosen over censored-aware
    operators because it can only under-match, never claim a match the
    data can't support.
  - **Byte-order tie-break.** `external_specimen_id COLLATE "C" DESC`
    pins decision 0004's lexical order; the column's default collation
    would pick a different "latest" specimen for ids differing in case or
    punctuation. Guarded by a SQL-text test (the Alpine test container is
    already bytewise, so a DB test alone can't catch a regression).
  - **Interrupted ≠ failed.** `ExecuteQueryTaskV1` returns
    `context.Canceled`/`DeadlineExceeded` from `CountMatchingCohort` as a
    Go error instead of a terminal `ERROR` result, so Phase 6/7 can resume
    an interrupted job; the SQL-builder failure is now logged.
  - **Executor enforces its own preconditions.** It re-runs the pure
    layers (version, structural, semantic) and maps failures to their
    `QueryResult` status, so a mis-ordered caller can't get an `OK` count
    for a v2 / `group_by` / non-decimal task. The whitelist layer needs D5
    and remains the caller's job (`ValidateQueryTaskV1`).
  - **Cleanup.** `parseCohortDate` removed in favour of
    `parseSchemaV1Date`; dropped the redundant `COUNT(DISTINCT)`
    (`UNIQUE (specimen_id, field_code)` already guarantees uniqueness) and
    the unreachable `count < 0` guard. Left open: collapsing the
    per-panel/per-condition correlated subqueries into one aggregate, and
    the remaining defensive re-checks in the repository, until there is a
    real registry size to measure against.
- 2026-09-21 (Phase 6): D4 checkpoint + resumable execution, per decision
  0005 (patient-ID chunks, checkpoint per chunk, suppression on the final
  total only, criteria hash, finished rows kept 7 days). `job_progress`
  table (migration 04) holds `job_id`, `criteria_hash`, `last_patient_id`,
  `running_count`, `status`, `updated_at`; cursor and count are one upsert.
  `PatientRegistryRepository.CountMatchingCohortChunk` counts the next N
  candidate patients after a cursor (candidates are chosen first, then
  ranked, so each patient is judged on all its in-range specimens).
  `usecases.NewCohortCountUseCase` runs the chunk loop, resumes a matching
  checkpoint, restarts on a different `criteria_hash`, serves a `done` job
  without querying, prunes finished rows older than 7 days (best-effort), and
  errors instead of looping if a cursor fails to advance.
  `ExecuteQueryTaskV1` now takes that use case and keys it by `job_id`;
  structural validation requires a non-empty `job_id` of at most 255
  characters. `JOB_CHUNK_SIZE` (default 5000) is in `conf` / `.env.example`.
  `CountMatchingCohort` (unchunked) stays as the oracle the chunk tests
  compare against. Proof: `TestCohortCount_InterruptedThenResumedEqualsUninterrupted`
  (real Postgres; third chunk cancelled, resume starts at the checkpoint and
  re-reads only the unfinished chunks), chunk-sum-equals-unchunked for sizes
  1/2/3/7/1000, and usecase tests for resume, done, criteria mismatch, empty
  final chunk, and error paths. Mutation-checked: changing the cursor
  comparison to `>=` fails both the chunk-sum and the resume tests. Left to
  Phase 7: wiring the use case and the 30s heartbeat's in-flight job IDs in
  DI; nothing constructs `NewCohortCountUseCase` outside tests yet.
- 2026-09-22 (Phase 7): `docs/decisions/0006-grpc-client-transport-security.md`
  records the transport-security decision this phase needed first: TLS
  required by default (system CA pool or `CONTROL_CENTER_CA_FILE`), mTLS
  auto-enabled when a client cert/key pair is configured, plaintext only via
  explicit `CONTROL_CENTER_INSECURE=true` (fails fast; mirrors the
  `ADMIN_BASIC_AUTH_USER/PASS` partial-config pattern for a one-sided
  cert/key pair). `internal/adapters/federated/client.NodeClient` dials with
  `grpc.NewClient` plus a documented `waitForReady` polling loop (the modern
  replacement for the deprecated `grpc.WithBlock`), registers, runs one
  Send-goroutine and one Recv-goroutine per connection (the concurrency
  pattern the gRPC stream API requires), heartbeats every 30s with a
  snapshot of in-flight `job_id`s, executes each `QueryTask` in its own
  goroutine via `ValidateQueryTaskV1` → `ExecuteQueryTaskV1`, and reconnects
  with jittered exponential backoff (half-to-full of the doubling interval,
  capped at `MaxBackoff`) on any stream loss. A `QueryTask` interrupted by a
  dropped connection sends nothing back - decision 0005's checkpoint means
  the next delivery of the same `job_id` resumes instead of restarting, so
  there is nothing to report yet. `UpdateAdvisory` is logged and stored in
  an `AdvisoryStore`, exposed read-only via `GET /admin/status` (decision
  0003; same admin Basic Auth as `/admin/query-fields`). The worker is
  optional: a blank `CONTROL_CENTER_ADDRESS` only logs a warning, since no
  control center exists yet to dial (decision 0001). `cmd/app.Run` waits for
  the worker to stop, bounded by the same graceful-shutdown timeout as the
  HTTP server. Proof: `internal/adapters/federated/client` unit tests
  against a real in-process `NodeControl` server over `bufconn` (not a
  hand-rolled substitute) covering register, execute-and-reply,
  version-gate-without-querying-D3, advisory storage, and
  reconnect-after-drop; all pass under `-race`. Also fixed a latent flaky
  test from Phase 6 (`TestCohortCount_ResumesFromCheckpointForSameCriteria`
  compared two `uuid.New()` values whose byte order isn't guaranteed, which
  the Phase 6 cursor-advance guard could reject about half the time) by
  switching to deterministic, ascending test UUIDs.
- 2026-09-22 (Phase 8): `internal/testsupport/controlcenterstub.Server` is a
  real `nodev1.NodeControlServer` (not a hand-rolled substitute for the
  client under test) serving over a loopback TCP port
  (`net.Listen("tcp", "127.0.0.1:0")`), chosen over `bufconn` so Phase 9 can
  boot the actual `client.NodeClient` against it with its real dialer -
  `client.TLSConfig{Insecure: true}` is decision 0006's documented opt-out
  for exactly this case. It supports one connected node at a time (all
  Phase 9 needs): `Registers()`/`Heartbeats()`/`QueryResults()` channels
  observe what the node sends, `SendQueryTask`/`SendUpdateAdvisory` push to
  whichever node is currently connected (blocking until one connects, up to
  the caller's `ctx`), and `DropConnection` force-closes the active stream
  to exercise Phase 7's reconnect path. `TestCmdProductionCodeDoesNotImportTestSupport`
  (`internal/domain/architecture_boundary_test.go`) makes "never import
  this from `cmd/`" a build-breaking check, not just the package doc
  comment. Proof: the package's own tests drive it with the generated
  `nodev1.NodeControlClient` directly (not `client.NodeClient`), covering
  register/heartbeat recording, task delivery and result recording,
  advisory delivery, no-connection error, connection drop ending the
  stream, and a second connection replacing the first - all passing under
  `-race`.
- Promote any phase-specific decision (e.g. checkpoint granularity,
  a specific mapping ambiguity) into `docs/decisions/` as that phase
  starts, per the pattern already used by 0001-0004.

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

Phase 8 complete — `internal/testsupport/controlcenterstub.Server`, a real
`NodeControl` gRPC server over a loopback TCP port, gives Phase 9 something
the actual `client.NodeClient` can dial out to and that can push
`QueryTask`/`UpdateAdvisory` and drop the connection on command. Guarded
against ever reaching a production binary by
`TestCmdProductionCodeDoesNotImportTestSupport`. Phase 9 (end-to-end
integration test wiring the real client, the stub, and the full app
together) is next.
