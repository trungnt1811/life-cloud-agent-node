# Hospital Local Governance API

Approval expiry uses `APPROVAL_EXPIRED` and shared wire reason
`APPROVAL_DEADLINE_REACHED=17`. The deadline is the first CP-persisted waiting
ACK, not a new local/reconnect timer. Expiry at that deadline wins over a late
approve command, including when capped by the absolute query deadline. A
restarted Node recovers an unacknowledged expiry as count-free historical proof
through its current synchronized session; it does not recount, request a grant
or export a held count. Once the ACK is durable the terminal row is no longer
selected by manual recovery. This does not implement a working-day calendar.

These private Node APIs persist hospital-local policy, approval and ledger state
for the opt-in `governed-cohort/v1` profile. The Node supports AUTO and MANUAL
release when `GOVERNED_EXECUTION_ENABLED=true`. Defaults remain OFF; the coordinated
two-Node synthetic demo VPS is now activated as recorded in the sibling CP
`docs/runbooks/governed-runtime-release.md`. The
flag requires governance sync, verified mTLS and durable PostgreSQL dependencies.
Local approval does not itself grant CP release authority. Legacy jobs keep their
existing field guard and k=5 behavior; changing this policy does not affect them.

## Access and Migration

Apply additive Node migrations 05 through 10 using `make migrate`
before starting this Node release. Keep the admin listener private. Use a
hospital-local BFF/internal tool with `ADMIN_BASIC_AUTH_USER/PASS`; do not put
Basic secrets in a browser, public Node endpoint or CP admin proxy. The configured
Basic identity is a service actor, not individual DPO/human approval evidence.

Both Basic credentials absent means these routes are not registered. Partial
credentials fail startup through the existing admin validation. Missing/invalid
credentials return `401`; a configured route without a usable store/node identity
returns safe `503`. There is no HTTP endpoint for installing a trusted snapshot.

| Request | Response |
| --- | --- |
| `GET /admin/policy` | `HospitalPolicyDTO`: version/hash, k, AUTO/MANUAL mode, field-use sets and sync metadata. Initial version 1/k=10/all fields disabled; no grant is implied. |
| `PUT /admin/policy` | `200` recorded policy snapshot. Requires `Idempotency-Key` and positive `expected_revision`, compared with policy `version`. Only AUTO/MANUAL and k >= 10 are accepted. |
| `GET /admin/availability` | Versioned local `paused` plus sync metadata. Starts version 1/false; independent of ONLINE and CP disable. |
| `PUT /admin/availability` | Required `paused` boolean and positive `expected_revision`, plus `Idempotency-Key`; local audit/command atomicity. New local revision requires CP ACK. Does not pause legacy V1. |
| `GET /admin/study-permits?page=1&page_size=20` | `items`, `total_count`, `page`, `page_size` plus sync metadata. Shows received CP scopes and any exact-version/hash local decision; empty while runtime metadata sync is OFF. |
| `PUT /admin/study-permits/{id}/acceptance` | `200 HospitalAcceptanceDTO`. Requires `Idempotency-Key`; only known active permit version/hash and a restricted subset of its fields/expiry can be accepted or declined. No automatic acceptance. |
| `GET /admin/outbound-events?page=1&page_size=20` | Bounded protected ledger with delivery state, job/query/event IDs, permit/policy references, protected preview/digest and optional receipt metadata. No raw checkpoint, principal or grant. Empty until stream execution is enabled later. |
| `GET /admin/approvals?page=1&page_size=20` | Pending MANUAL approvals after the CP has acknowledged the waiting stage; protected preview, definition/permit/policy references, deadline and revision. |
| `POST /admin/approvals/{job_id}/approve` | Node Basic, UUID `Idempotency-Key` and `{"expected_revision":N}`. Approval, audit and job revision commit atomically; approval is not itself egress. |
| `POST /admin/approvals/{job_id}/decline` | Same concurrency/idempotency contract; terminal local decline. No client-provided approver identity or free-form reason. |

All bodies reject unknown JSON fields/enums, malformed JSON and trailing JSON.
The approval deadline is the first CP-persisted waiting stage plus 48 elapsed
UTC hours, capped by the immutable query/permit lifetime. Transport delay does
not restart or shorten that clock using the local event timestamp. Reconnect
and CP/Node restart preserve the deadline, protected payload and command history;
they do not approve or recount a held result. A committed cancel/revoke/pause
stops held work; local approval still requires a fresh CP release grant.

The endpoint limit is 64 KiB, including chunked bodies (`413`). Public errors
never include parser/SQL internals. k is a canonical decimal string bounded by
MaxInt64, not a floating-point number; versions/revisions are integers.

## Policy Example

Use the Node's private listener or an authenticated SSH tunnel. Credentials below
are placeholders; load them server-side without shell tracing or screen capture.

```bash
export NODE_ADMIN_URL='http://127.0.0.1:8080'
export NODE_ADMIN_USER='<hospital-service-user>'
export NODE_ADMIN_PASS='<hospital-service-password>'

curl --fail-with-body -sS \
  -u "$NODE_ADMIN_USER:$NODE_ADMIN_PASS" \
  "$NODE_ADMIN_URL/admin/policy"

# Use the version from GET as expected_revision; one key per logical update.
curl --fail-with-body -sS -X PUT \
  -u "$NODE_ADMIN_USER:$NODE_ADMIN_PASS" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: local-policy-demo-1' \
  "$NODE_ADMIN_URL/admin/policy" \
  -d '{
    "expected_revision": 1,
    "local_k": "15",
    "release_mode": "AUTO",
    "enabled_field_codes": ["MCH", "MCV"],
    "filter_field_codes": ["MCH", "MCV"],
    "panel_field_codes": []
  }'
```

Filter/panel sets must be subsets of enabled fields; values are normalized sorted
unique schema codes. This sets future governed policy only. It does not turn on
governed execution, allow grouped queries, change legacy suppression or grant
access to hospital data. The policy hash recipe/fixtures are pinned in the
sibling CP `requirements/governed_cohort_contract.md`.

## Acceptance Command

The shape below is available only for a scope already received through the
verified CP stream. With metadata sync OFF, a stock Node has no scope;
inventing the ID/hash returns a safe validation error. Do not insert production
snapshots or acceptance directly into DB to simulate consent.

```json
{
  "permit_version": 1,
  "permit_hash": "<hash-from-the-trusted-CP-snapshot>",
  "decision": "ACCEPTED",
  "allowed_field_codes": ["MCV"],
  "expires_at": "2026-11-01T00:00:00Z",
  "expected_revision": 0
}
```

Decision is `ACCEPTED` or `DECLINED`. Zero expected revision means no previous
local decision for that permit version; otherwise use its local revision.
Acceptance cannot widen CP scope/expiry or replace a fresh decision for a new
version/hash. Tenant is derived from the hash-bound snapshot, not request JSON.

## Replay and Synchronization

State, mandatory audit and actor/key-scoped immutable command response commit in
one PostgreSQL transaction. Identical retries return their original response
before reading current state; changed intent or stale revision returns `409`.
Audit/commit failure returns `503` and rolls back writes. A replay response is
historical command evidence: GET the current policy/list for authoritative state.

`local_revision` changes with local policy/acceptance/pause/invalidation. `snapshot_revision` and
`acknowledged_revision` describe persisted protocol state; `cp_synchronized`
requires an exact current ACK under a nonempty matching session epoch. The
runtime clears stored authority on a new/disconnected connection; old ACKs cannot
restore it. Set `GOVERNANCE_SYNC_ENABLED=true` only with verified mutual TLS,
configured CP address/client certificate/key and a compatible CP with migration
015. Default is false. A new permit version never inherits old acceptance.
Snapshots/reports/invalidations/ACKs persist before transmission. No result-received claim is inferred
from an HTTP `200` or software capability.

## Availability and Protected Ledger

Read `/admin/availability`, then send the returned version as `expected_revision`:

```json
{"paused":true,"expected_revision":1}
```

Resume is a separate command/key with `paused:false` and the latest version.
GET current state after replay: the immutable command response is historical.
Wait for `cp_synchronized:true` before claiming CP knows about the new pause;
local new-work/grant guards already reject while locally paused. Do not label a
legacy query as hospital-paused or claim mid-flight stream enforcement is active
in this pre-activation release.

Ledger states are `PREPARED`, `SENT_UNCONFIRMED`, `RECEIVED`, `REJECTED`.
An attempted send is recorded before egress and remains uncertain until a bound
CP receipt. The digest covers disclosed payload only. Exact zero is `"0"`;
suppressed output is `value:null`, bounds `"1"` to `effective_k-1`.
All count/bound/k fields are decimal strings. Raising k re-protects old visible
data; lowering it cannot unmask an already suppressed value. Ledger history is
immutable by event and delivery cannot regress. The local use cases enforce
these invariants, but actual CP grants and stream release remain OFF.

## Verification

```bash
go test ./internal/domain/entities ./internal/domain/usecases \
  ./internal/adapters/federated/wire -run TestGovernedHospital -count=1
go test ./cmd/app ./internal/adapters/repositories \
  -run TestGovernedHospital -count=1 -timeout 5m
```

Tests use PostgreSQL/Testcontainers and trusted synthetic fixtures for
acceptance. They cover concurrency, replay, rollback, tenant/hash collisions,
scope restrictions, stale ACKs, safe binding and mapper validation. Repository
recreation alone is not release proof. The CP crossrepo process test additionally
proves metadata reconnect/restart/revoke/ACK, pause durability and MANUAL
hold/approve/release across two agents. Bounded lifecycle process tests also pass
for held CP/Node restart, cancel/revoke before release, concurrent approve/decline
and execution disconnect followed by a separate retry. CP PostgreSQL tests cover
approval expiry, concurrent workers and audit rollback. Governed multi-CP
failover, local policy/pause races, expiry across both processes and
browser/real-hospital/production acceptance remain separate gates. Generated
`docs/swagger.json` is the Node HTTP field reference. The OFF foundation archive
`s03-20261007-r1` is deployed to both demo Nodes with migrations 01-09; legacy
mTLS smoke passed. The later governed runtime now runs on both demo Nodes with
migrations 01-10 and source-pinned `governed-20261007T213840Z-node-r3` images.
Deterministic decline and mixed AUTO/MANUAL tests supplement the lifecycle tests;
the internal LOCAL_DECLINED reason is transmitted as APPROVAL_DECLINED. Full
Node-originated expiry wire/process acceptance remains open.
Hospital-owned BFF/browser acceptance remains unverified.
