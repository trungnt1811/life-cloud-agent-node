# Execution Plan: Governed Cohort Node Integration

Date: 2026-10-06

## Status

Active. Hash/wire/privacy pre-activation baseline verified; durable governance
and execution remain open. This is Node-specific handoff/recovery context, not
a second independent specification or a claim of real-hospital sign-off.

## Outcome

Implement the Node portion of the sibling Control Plane's governed-cohort plan
without allowing incompatible/legacy execution or exporting hidden raw counts.
Coordinate both repositories before activating a public governed query.

## Context

Authority: the user-approved CP
`requirements/demo_prd_backend_alignment_plan.md` and its normative
`requirements/governed_cohort_contract.md`. Decisions 0001/0002/0004/0005/0006
continue to govern legacy execution. No IAM/FE application implementation or
shared VPS operation is part of this change.

## Scope

In scope: separate governed wire variants, canonical hashes, protected-only
payloads, independent local policy/acceptance and durable AUTO release/receipts.
Out of scope for the present baseline: profile activation, MANUAL approvals,
hospital BFF/browser authentication, real-hospital privacy approval and exports.

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
- Keep the profile OFF and the legacy path unchanged during rollout. No Node
  schema migration or shared deployment was performed by this baseline.
- Event digest covers protected disclosure only; suppressed raw counts must
  never enter errors, wire, ledger payloads or auxiliary digests.

## Progress

- [x] Separate definition hashes from legacy checkpoint fingerprints; shared vectors.
- [x] Add coordinated protobuf variants, pin descriptor parity and preserve legacy numbers.
- [x] Refuse unsupported governed/unknown streams; advertise no governance capability.
- [x] Add protected-count helpers/wire encoding and eight independent golden vectors.
- [ ] Persist versioned local policy, CP snapshots, acceptance and local audit atomically.
- [ ] Implement snapshot/report/ACK sync, session fencing and invalidation application.
- [ ] Implement durable AUTO output hold/ledger and grant/receipt reconciliation.
- [ ] Verify PG concurrency/restart plus actual two-Node governed release before activation.

## Decisions

- 2026-10-06: CP's selected governed k=10 floor is opt-in only; legacy k=5 and
  checkpoint hashes stay unchanged. The running client never calls the new
  protection helper or advertises the profile until S03/S04 gates pass.
- Protected JSON uses decimal strings/nulls, explicit exact zero or positive
  bounds, and `protected-cohort-payload/v1\n` SHA-256; see pinned fixtures.
- Local Basic service authority and the synthetic 48 elapsed UTC hours are
  reviewed engineering defaults in the CP contract, not individual DPO
  authentication, working-day semantics or clinical/legal approval.

## Validation

- RED: real bufconn client tests showed all new/unknown messages silently ignored;
  GREEN: close/reconnect, no query counter or legacy result, no advertised profile.
- Wire RED: missing variants/exclusive unions in CP descriptor assertions;
  GREEN: coordinated additive descriptors pinned to
  `sha256:5d9e2d74a045dd4e181b574f88f41a59c2d4666e048995704c6b91fff2abc1c6`.
- Protected helper/mapper tests began with missing-symbol RED builds; boundary,
  overflow, raised-k and exact-zero/suppression assertions are GREEN.
- `go test ./... -count=1 -timeout 15m`: PASS including actual PostgreSQL tests.
- `go test -race ./gen/lifecloud/node/v1 ./internal/domain/types ./internal/adapters/federated/wire ./internal/adapters/federated/client -count=1 -timeout 3m`: PASS.
- `make lint`: PASS. No CI/hook/branch-protection configuration changed or verified externally.
- `make proto` regeneration is byte-identical by before/after generated file
  SHA-256. Commit both repositories' intentional proto changes before using
  clean-HEAD `proto-check`; no real git index was changed.
- CP crossrepo hash/protocol/privacy parity and actual legacy agent mTLS flow: PASS.

## Result

Pre-activation baseline is buildable/tested. Full governed Node integration is
not complete; retain this active plan until durable acceptance/release/recovery
proof passes. Generated APIs alone never authorize dispatch or release.
