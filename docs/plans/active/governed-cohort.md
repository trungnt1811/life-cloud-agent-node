# Execution Plan: Governed Cohort Node Integration

Date: 2026-10-06

## Status

Active. Hash/wire/privacy baseline, local policy/acceptance/pause, opt-in live
metadata sync/invalidation and protected-ledger foundations are verified. CP
internal grant/receipt/reconciliation transactions and owner reads now pass.
Opt-in AUTO/MANUAL release and bounded lifecycle tests now pass locally with
two real Agent processes and CP over mTLS. Held CP/Node restart, cancel/revoke,
concurrent approve/decline and execution-disconnect/partial coverage are verified.
The coordinated synthetic demo VPS is now activated with Node migrations
01-10 and both runtime flags ON; source defaults remain OFF. See the sibling
CP `docs/runbooks/governed-runtime-release.md` for pinned images and evidence.
Governed multi-CP failover and the
broader expiry/pause/policy-tightening matrix remain open. This is Node-specific
handoff/recovery context, not a second specification or real-hospital sign-off.

## Outcome

Implement the Node portion of the sibling Control Plane's governed-cohort plan
without allowing incompatible/legacy execution or exporting hidden raw counts.
Coordinate both repositories before activating a public governed query.

## Context

Authority: the user-approved CP
`requirements/demo_prd_backend_alignment_plan.md` and its normative
`requirements/governed_cohort_contract.md`. Decisions 0001/0002/0004/0005/0006
continue to govern legacy execution. No IAM/FE application implementation is
part of this change. The latest user request authorizes coordinated deployment
and bounded tests on the existing demo VPS instances after validation. The
runtime checkpoint is now deployed; no production or browser acceptance is implied.

2026-10-07 deployment fixes: AUTO uses the shared mixed-query lifetime while
retaining its independent 30-second execution budget. LOCAL_DECLINED persists
locally but maps to the shared APPROVAL_DECLINED wire reason. A deterministic
wire test failed before the fix; two-Agent decline, concurrency and mixed-mode
process tests now pass. Node full tests, default lint and scoped race tests pass.
Node-originated approval expiry now uses the coordinated additive reason 17.
RED/GREEN tests prove terminal serialization and bounded historical ACK recovery
after absolute query expiry. PostgreSQL tests race approval with expiry and
verify a single expiry audit, no outbound intent and no rescan after durable ACK.
The CP's real two-Node mTLS process test verifies worker-first expiry followed by
Node restart/reconnect twice without recounting or release. Its shortened stored
deadline exists only in isolated test databases; production remains 48 elapsed
UTC hours. The broader uncertain-egress/multi-CP matrix remains open.

## Scope

In scope: separate governed wire variants, canonical hashes, protected-only
payloads, independent local policy/acceptance and durable AUTO/MANUAL
release/receipts with coordinated opt-in CP activation. Hospital BFF/browser
implementation, real-hospital privacy approval and CP-owned export rendering
are outside this Node repository's delivery.

## Approach

Follow CP S00/S03 with RED/GREEN tests, then actual PG/process tests for durable
policy/acceptance/ledger. S04 composes the public two-Node AUTO vertical slice.
Code generation/fixtures must be coordinated; do not advertise partially
implemented governance, reinterpret a new task as legacy or use the existing
k=5 suppression helper for governed work.

## Risks And Recovery

- Generated types are software contracts, not proof of accepted scope or a grant.
- Closing unknown control streams intentionally fails closed; a server sending
  unsupported messages will reconnect repeatedly until corrected.
- Keep the profile OFF and the legacy path unchanged during rollout. This
  checkpoint adds migrations 05/06/07; apply them before deploying the new Node. No
  shared deployment was performed. Local policy k=10 does not change legacy k=5.
- Event digest covers protected disclosure only; suppressed raw counts must
  never enter errors, wire, ledger payloads or auxiliary digests.

## Progress

- [x] Separate definition hashes from legacy checkpoint fingerprints; shared vectors.
- [x] Add coordinated protobuf variants, pin descriptor parity and preserve legacy numbers.
- [x] Refuse unsupported execution/release and unknown streams; default advertisement remains empty.
- [x] Add protected-count helpers/wire encoding and eight independent golden vectors.
- [x] Persist versioned local policy, CP snapshots, acceptance and local audit atomically.
- [x] Add private Basic policy/acceptance APIs with derived actor, command replay, safe errors and OpenAPI.
- [x] Pin shared permit/local-policy hashes and strict snapshot/report mapper primitives.
- [x] Implement opt-in snapshot/report/ACK sync, session fencing and invalidation application; require verified mutual TLS and durable dependencies.
- [x] Add durable local pause/resume and protected-only ledger foundations with private reads, monotonic delivery and mandatory audit transactions.
- [x] Re-protect under a fresh event after confirmed LOCAL_POLICY_CHANGED rejection; reject contradictory hydrated release history.
- [x] Journal EXECUTING/READY stages and durable CP ACKs without extending local/CP deadlines; strict task/grant/receipt/result mappers.
- [x] Apply query-scoped cancellation/expiry commands atomically before ACK, preserve permit scope, and refuse canceled task/preparation/fresh-send authority across restart.
- [x] Implement durable AUTO/MANUAL output hold/ledger and grant/receipt reconciliation.
- [x] Verify PG concurrency/restart plus bounded actual two-Node governed release locally; no deployment activation is implied.
- [ ] Complete governed multi-CP failover, process-level expiry and the broader pause/policy-tightening/uncertain-egress matrix.

## Decisions

- 2026-10-07 live sync/ledger checkpoint: `GOVERNANCE_SYNC_ENABLED` defaults
  false and, when true, announces metadata compatibility only during this OFF
  rollout. Public execution/grants remain unavailable. CP report mirrors and
  scoped invalidation ACKs are durable. Pause is local, separate from ONLINE
  and does not apply to legacy work. Ledger use cases persist protected payloads
  before intent/send, retain uncertainty until receipts, and cannot regress
  terminal delivery. They are not wired to the stream executor yet.

- 2026-10-07: local policy starts at k=10/AUTO with no enabled fields; MANUAL,
  pause and release are unavailable. Accepted metadata requires a trusted
  version/hash snapshot. There is no HTTP snapshot import or auto-accept.
  New local APIs do not advertise governance or alter legacy field guards.

- 2026-10-06: CP's selected governed k=10 floor is opt-in only; legacy k=5 and
  checkpoint hashes stay unchanged. The running client never calls the new
  protection helper or advertises the profile until S03/S04 gates pass.
- Protected JSON uses decimal strings/nulls, explicit exact zero or positive
  bounds, and `protected-cohort-payload/v1\n` SHA-256; see pinned fixtures.
- Local Basic service authority and the synthetic 48 elapsed UTC hours are
  reviewed engineering defaults in the CP contract, not individual DPO
  authentication, working-day semantics or clinical/legal approval.

## Validation

2026-10-07 bounded lifecycle/recovery checkpoint:

- CP crossrepo tests kill/restart Node and CP during a MANUAL hold, preserving
  payload digest, execution start and the CP-persisted approval deadline with no
  automatic release/recount. Cancel/revoke before release never obtains a new
  grant; concurrent approve/decline has one winner. Execution disconnect yields
  partial coverage; retry is a separate query, not a changed old result.
- RED/GREEN Node regressions: delayed CP approval clock versus local event time,
  recovery DB error closing the stream, held actor invalidation and rebinding
  invalidated work after restart without recounting or egress. Uncertain sends
  remain receipt-reconciliation work, not an invalidation overwrite.
- Both full suites and default lint pass locally. Race tests cover the Node
  entities/client and actual CP PostgreSQL deadline/audit/worker transactions.
  No feature flag was enabled on a VPS; broader failover/expiry/local policy
  races, hospital BFF acceptance and production sign-off remain open.

2026-10-07 AUTO actor / receipt-only recovery checkpoint:

- `GOVERNED_EXECUTION_ENABLED` defaults false and requires metadata sync,
  verified mTLS and durable governance/job/counter dependencies. Actor tests
  prove durable stage ACK before counting, protected ledger/audit before wire,
  active pause interruption and no result enqueue on mandatory-audit failure.
- Grant denial persists separately from receipt; retryable denial rotates an
  intent only after durable proof. Terminal stages persist/rebind across
  reconnect without rewriting historical authority or execution clocks.
- Additive `receipt_lookup=9` carries no count. Uncertain send requests the
  original receipt instead of retransmitting protected bytes after expiry.
  CP atomically fences uncommitted old-session/expired events before delayed
  results can commit. A live grant without receipt remains uncertain.
- After ACK sync, a once-per-connection bounded keyset scan reconciles
  SENT_UNCONFIRMED history even when CP no longer dispatches the query.
  Migration 09 adds its partial index; apply 05-09 with CP 013-018.
- RED/GREEN: absent lookup/recovery methods, unsupported lookup transport,
  storage-field hash mismatch, terminal epoch not rebound, and borrowed task
  criteria mutated. Both full suites, both linters, focused runtime/receipt/PG
  races (count=3), shared descriptor/privacy/hash parity and actual two-Node
  metadata/cancel/restart plus legacy mTLS query pass after migration 09.
- Earlier full CP/Node suites and actual two-Node cancel/metadata restart proof
  passed after migration 08. Those results do not prove this new AUTO release.
  No new VPS deployment, actual AUTO two-Node gate, MANUAL flow or FE/browser
  acceptance is claimed. Public governed submission and CP receiver remain OFF.

2026-10-07 stage/cancellation checkpoint:

- Stage ACK PG/race proof covers concurrent identical ACKs, one audit,
  audit rollback and recreated use cases; conflicting CP execution deadlines
  cannot mutate the recorded clock. Task/release wire helpers reject unknown
  fields, invalid scope and unsupported terminal claims. They remain unwired
  into the counter/release executor.
- Actual two-Node metadata process test in CP now also creates an internal
  gated query, cancels through real HTTP, checks durable local query fences
  and CP command ACKs, then verifies Node restart durability. This does not
  activate or prove governed AUTO execution.
- Local query invalidation PG/race tests cover concurrent replay, epoch fencing,
  changed command intent and mandatory-audit rollback. Permit acceptance is
  not revoked by a query cancel. Unknown/mismatched wire scopes are refused.
- Migration 08 adds nullable query/command IDs to local audit, including job
  lifecycle query references. Observed RED: the audit correlation assertion
  failed with a missing query_id column; the migrated implementation passes.
  No historical reference is fabricated. Apply 05-08 before this Node release
  and CP 015-018 before the coordinated CP release.
- Full Node suite passed before the final audit-reference extension; its
  focused PG/entity/use-case race tests pass after that extension. Final full
  suite and crossrepo rerun remain delivery checks until recorded below.
- No shared VPS deployment/browser acceptance is claimed. Public governed
  execution remains OFF; cancellation fences do not prove an active counter
  observes them until the runtime executor is composed and tested.

2026-10-07 live metadata and local-ledger checkpoint:

- Full Node and CP `go test -p 1 ./... -count=1 -timeout 15m`: PASS with actual PG.
- Node scoped PG/HTTP/entity/type race tests and scoped CP mirror/gateway races:
  PASS. Both linters pass before the final documentation/test-only additions;
  final lint remains part of the delivery check, not assumed from that run.
- Actual crossrepo `TestGovernedHospitalTwoNodeSynchronizationSurvivesProcessRestarts`:
  PASS. Two binaries/private databases/mTLS, CP and Node process restarts,
  new-version explicit acceptance, revoke/outbox ACK and persistent pause/resume.
- RED/GREEN: missing availability route, missing protected JSON keys accepted,
  terminal ledger overwrite, inability to obtain fresh intent after confirmed
  grant expiry, and CP acceptance revision comparison across permit versions.
- PG verifies concurrent release intents produce one event, new use-case hydration
  retains protected suppression and SENT_UNCONFIRMED, receipt replay writes one
  audit, and audit failure rolls back egress intent/grant/ledger. This is local
  transaction proof, not an actual stream release or CP commitment proof.
- Local availability commands use required bool/revision pointers, stable key,
  service actor and audit atomicity. Outbound reads are Basic-only, paginated,
  decimal-string disclosure; no raw checkpoint, principal or grant is exposed.
- Apply Node migrations 05/06/07 and CP 015 before opting into metadata sync.
  No new VPS deployment or browser acceptance has run yet.

2026-10-07 local persistence/API checkpoint:

- Observed missing-symbol RED for new domain/use-case/repository/mapper assertions;
  HTTP/PG RED returned 404 until routing/DI were wired.
- DTO tenant/hash-collision and CP member-bound RED assertions exposed defects
  in the new implementation, then passed after correction. No deployed leak
  or production incident is claimed by these synthetic tests.
- Actual PG verifies one concurrent revision winner, concurrent identical
  commands with one audit, immutable replay and multi-write rollback under
  audit failure. Recreated repositories retain state/commands; stale ACKs
  cannot restore fenced authority. No process-level governed restart is proven.
- HTTP validates Basic identity, unknown/widened scopes, stale revisions,
  unsupported MANUAL/k<10, safe malformed/unknown/trailing JSON and 413.
- HTTP RED also exposed an omitted expected_revision being treated as initial
  revision zero. Required pointer fields now reject omitted/null revision before
  the use case; explicit zero remains valid only for initial acceptance.
- Full Node `go test ./... -count=1 -timeout 15m`: PASS, including actual PG
  and existing legacy integration. Scoped `-race` domain/DTO/wire tests: PASS.
- `make lint`, `make mockgen` and `make swagger`: PASS. Shared CP/Node hash
  parity tests: PASS. No profile/proto/CI/hook or shared VPS operation changed.
- Required `make test-postgres-repositories`, actual HTTP/repository race tests,
  `make proto-check` and template identity check: PASS. Node Swagger/mocks
  regenerate byte-identically; include intentional generated diffs in the
  eventual commit for normal clean-index CI checks. Real git index unchanged.

Prior 2026-10-06 baseline:

- RED: real bufconn client tests showed all new/unknown messages silently ignored;
  GREEN: close/reconnect, no query counter or legacy result, no advertised profile.
- Wire RED: missing variants/exclusive unions in CP descriptor assertions;
  GREEN: coordinated additive descriptors pinned to
  `sha256:95d051955ed00c663572a06c8687e2c7b6fdef18b395fea92e068ae9c88002ac` (updated by the receipt-only recovery checkpoint below).
- Protected helper/mapper tests began with missing-symbol RED builds; boundary,
  overflow, raised-k and exact-zero/suppression assertions are GREEN.
- `go test ./... -count=1 -timeout 15m`: PASS including actual PostgreSQL tests.
- `go test -race ./gen/lifecloud/node/v1 ./internal/domain/types ./internal/adapters/federated/wire ./internal/adapters/federated/client -count=1 -timeout 3m`: PASS.
- `make lint`: PASS. No CI/hook/branch-protection configuration changed or verified externally.
- `make proto` regeneration is byte-identical by before/after generated file
  SHA-256. Commit both repositories' intentional proto changes before using
  clean-HEAD `proto-check`; no real git index was changed.
- CP crossrepo hash/protocol/privacy parity and actual legacy agent mTLS flow: PASS.

2026-10-07 deployment and initial-refusal checkpoint:

- Foundation archive `s03-20261007-r1` deployed to both demo Nodes; migrations
  01-09 completed, sync/execution remain false. Private listeners/TLS/DB mounts
  preserved; both Nodes ONLINE with the archive-labeled agent version.
- CP query `3d667521-2a10-4f45-86eb-ab7218466169` completed with two jobs and
  synthetic count 1901. Replay/conflict, owner evidence, legacy result-view
  fallback and private admin refusal passed. This is legacy local-IAM evidence,
  not governed release or browser acceptance.
- Working-tree-only initial refusal: observed PG RED returned conflict instead
  of a durable DECLINED stage; wire RED refused sequence 1/no-clock ACK. GREEN
  persists pause/scope/acceptance/invalidation refusal with one stage, no execution
  timestamps/count/ledger and mandatory audit. Duplicate, restart, audit failure,
  malformed/foreign task and invented-clock negatives pass.
- CP RED accepted invented initial payload digest/duration; GREEN refuses both.
  CP PG/stream tests prove terminalization without execution clock/count receipt.
  Runtime refusal keeps a healthy stream open and never calls the counter.
  Scoped Node PG/entity/wire/runtime races passed count=3.
- Both full `go test ./... -count=1 -timeout 15m` suites and both linters passed
  after this change. No validation was skipped or weakened.
- Deployment hashes, private backups and rollback boundaries live in the CP
  `docs/runbooks/governed-foundation-release.md`. Initial-refusal code is newer
  than the deployed OFF archive; do not claim it is live.

## Result

Pre-activation baseline is buildable/tested. Full governed Node integration is
not complete; retain this active plan until durable acceptance/release/recovery
proof passes. Generated APIs alone never authorize dispatch or release.
See [local API reference](../../hospital-governance-api.md). Next: CP durable
dispatch/reconciliation, public OFF-gated composition, then actual two-Node S04
AUTO execution before activation. MANUAL/catalog/export and hospital-owned
BFF/browser acceptance remain separate unclosed slices.
