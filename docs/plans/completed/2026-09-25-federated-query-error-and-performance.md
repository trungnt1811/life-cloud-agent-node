# Federated Query Error And Performance Review

Date: 2026-09-25

## Status

Completed

## Outcome

Node and control-plane query errors remain observable and reach a terminal
state or an explicit retry path. Reduce measured avoidable database work while
preserving schema-v1 results, resume behavior, and session fencing.

## Context

- `docs/decisions/0005-job-checkpoint-granularity.md`
- `docs/decisions/0006-grpc-client-transport-security.md`
- Sibling `life-cloud-control-plane/requirements/implementation_plan.md`
- Current node and control-plane query, worker, and gateway code and tests.

## Scope

- Fix inner context errors that leave a live-stream job unanswered.
- Retain a local diagnostic for failed cohort execution.
- Propagate fatal federated-worker errors to the application lifecycle.
- Measure and reduce avoidable control-plane dispatch polling work.
- Measure and improve the local cohort chunk query when evidence supports it.
- Review checkpoint pruning against decision 0005.

## Approach

Write focused failing tests for each behavior change. Run focused tests after
the fix, then repository checks and the cross-repository mTLS test. Use
representative ephemeral PostgreSQL data and `EXPLAIN (ANALYZE, BUFFERS)` to
compare query candidates before changing SQL or indexes.

## Risks And Recovery

- Retry changes can alter job terminal state; preserve stream-loss resume and
  check both a live stream and a canceled stream.
- Polling changes must preserve PostgreSQL session fencing and job ordering.
- SQL changes must return the same cohort counts, cursor, and tie-break result.
- Revert each small diff if its focused proof fails; no production state is
  changed by this work.

## Progress

- [x] Add failing tests for error propagation and worker lifecycle.
- [x] Fix and validate error handling, including HTTP shutdown after worker failure.
- [x] Reduce control-plane session ownership reads from once per candidate job to once per node per dispatch cycle; dispatch still checks ownership before sending.
- [x] Measure and optimize cohort SQL. On an isolated PostgreSQL 15 dataset with 20,000 patients and 60,000 specimens, `EXPLAIN (ANALYZE, BUFFERS)` showed the 5,000-patient chunk query at 34.8 ms with window ranking, 25.5 ms with lateral lookup on the existing index, 30.8 ms with ranking and an added index, and 20.6 ms with lateral lookup and that index. Keep the SQL-only change to avoid an extra write-path index for a smaller incremental gain; this synthetic case does not establish production latency.
- [x] Preserve checkpoint pruning at next-job start, as required by accepted decision 0005.
- [x] Run cross-repository and repository validation.

## Decisions

- 2026-09-25: Preserve decision 0005's cleanup-at-next-job rule unless a
  documented authority changes it; optimize within that constraint.
- 2026-09-25: Use the existing specimen index with a lateral latest-specimen
  lookup. An additional composite index helped the synthetic read query but
  was not adopted without write-path evidence.

## Validation

- Agent Node: `make test` passed; focused `go test ./internal/server ./cmd/app ./internal/adapters/federated/client -count=1` passed after the final shutdown change; `make lint`, `make template-identity-check`, and `git diff --check` passed.
- Control Plane: `make test`, `make lint`, `make template-identity-check`, and `git diff --check` passed.
- Cross-repository: `go test -tags=crossrepo ./tests/crossrepo -run '^(TestAgentNodeCompletesControlPlaneQueryOverMTLS|TestProductionAgentsFederateRecoverAndLoadOverMTLS)$' -count=1 -timeout=8m` passed.
- PostgreSQL measurement: an isolated Testcontainers PostgreSQL 15 instance with synthetic 20,000 patients and 60,000 specimens; timings in Progress are single `EXPLAIN (ANALYZE, BUFFERS)` observations, not a production benchmark.

## Result

- Interrupted local execution now closes the live stream so the control plane can redispatch the checkpointed job after reconnect.
- Non-context execution failures retain local diagnostics and send a generic wire `ERROR`; fatal worker errors reach the application and HTTP shuts down gracefully.
- Control Plane takes one session snapshot per node per dispatch cycle, with a fresh ownership check at send time.
- The chunk query uses a lateral latest-specimen lookup on the existing index. Results, cursor progression, and byte-order tie breaking remain covered by integration tests.
- No migration was added. Cleanup timing remains as accepted by decision 0005.
