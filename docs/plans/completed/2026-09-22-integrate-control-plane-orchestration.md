# Execution Plan: Integrate Control Plane Orchestration

Date: 2026-09-22

## Status

Completed

## Outcome

The control plane accepts a schema-v1 cohort count request, resolves eligible
connected nodes, creates deterministic jobs atomically, dispatches from durable
PostgreSQL state, ingests terminal protected results, preserves suppression
metadata during aggregation, and exposes the lifecycle through HTTP.

A tagged cross-repository test builds and runs the production
`life-cloud-agent-node` binary against the production control-plane gateway over
mTLS. It verifies registration, dispatch, result ingestion, persistence, and
final aggregate completion.

## Context

- `docs/decisions/0002-federated-query-contract-and-execution.md`
- `docs/decisions/0004-grpc-wire-contract-schema-v1.md`
- `docs/decisions/0005-job-checkpoint-granularity.md`
- `api/proto/lifecloud/node/v1/node_control.proto`
- Sibling repository `life-cloud-control-plane/requirements/implementation_plan.md`

## Scope

Implemented:

- Aligned the canonical operation to `COUNT_MATCHING_COHORT` and QueryTask v1.
- Added typed cohort validation, explicit state transitions, capability-based
  target resolution, and atomic query/target/job persistence.
- Added a PostgreSQL polling dispatch/reconciliation worker.
- Connected heartbeat acknowledgements and QueryResult ingestion to the domain
  lifecycle, including idempotency, rejection audit, timeout, partial result,
  and suppression-aware aggregation behavior.
- Added query catalog/create/status/jobs HTTP endpoints and generated Swagger.
- Added the cross-repository production client/gateway mTLS proof.

Deferred by design:

- Redis Streams; PostgreSQL remains the durable dispatch source.
- Query schema v2, grouped aggregates, arbitrary SQL, and patient-level results.
- Production certificate issuance and external PKI integration.

## Progress

- [x] Align canonical query contract and catalog.
- [x] Implement target resolution, state transitions, and atomic job creation.
- [x] Implement durable dispatch and recovery worker.
- [x] Implement result ingestion and suppression-aware aggregation.
- [x] Add HTTP handlers, routes, DTOs, and Swagger.
- [x] Add and pass cross-repository mTLS end-to-end proof.
- [x] Run repository-required validation in both repositories.

## Decisions

- 2026-09-22: `COUNT_MATCHING_COHORT` is the first canonical job because it
  represents QueryTask v1 without implying a diagnosis.
- 2026-09-22: Redis is deferred; PostgreSQL polling is the authoritative first
  dispatch implementation.
- 2026-09-22: A suppressed node returns count zero, and any suppressed node
  makes the aggregate explicitly inexact.
- 2026-09-22: Cross-repository proof remains black-box at the process boundary
  to avoid a module dependency cycle between the two services.

## Validation

Control plane:

```text
go test ./...                                                      PASS
make lint                                                         PASS
make swagger                                                      PASS
make proto-check                                                  PASS
make template-identity-check                                      PASS
go test -race ./internal/adapters/services/nodegateway/grpc \
  ./internal/domain/usecases ./internal/workers \
  ./internal/delivery/http/router                                 PASS
go test -tags=crossrepo ./tests/crossrepo \
  -run TestAgentNodeCompletesControlPlaneQueryOverMTLS -count=1 -v PASS
```

Agent node:

```text
go test ./...  PASS
make lint      PASS
make proto-check PASS
make template-identity-check PASS
```

The cross-repository test used PostgreSQL 15 in Testcontainers and generated a
temporary CA, server certificate, and node client certificate. The final query
was `COMPLETED` with one succeeded node job and a persisted exact aggregate.

## Result

All six planned integration steps are implemented and validated. The next
independent slices are cancellation, metrics/readiness expansion, or Redis
transport if an operational requirement justifies it.
