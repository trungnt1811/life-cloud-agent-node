# Execution Plan: Implement gRPC Proto v1

Date: 2026-09-16

## Status

Completed

## Outcome

Checked-in `.proto` source and generated Go stubs for `lifecloud.node.v1`
(`NodeControl.Connect`), with CI `proto-check` and tests proving the demo
fixture compiles to schema-v1 `QueryTask`.

## Context

- Authority: `docs/decisions/0004-grpc-wire-contract-schema-v1.md`
- Field dictionary: `docs/product/query-field-dictionary.md`
- Demo fixture: `life-cloud/data/federated_mvp/benchmark/query_definition.json`

## Scope

In scope:

- `api/proto/lifecloud/node/v1/node_control.proto`
- Generated stubs under `gen/lifecloud/node/v1/`
- Makefile `proto` / `proto-check`
- Compile helper + tests in `internal/adapters/federated/wire/`
- CI and status doc updates

Out of scope:

- gRPC server/client runtime
- Four-layer validation, query execution, suppression
- `enabled_query_fields` admin REST

## Approach

1. Write proto 1:1 against decision 0004.
2. Add Makefile codegen (pinned plugins, committed output).
3. Add fixture compile helper and structural v1 tests.
4. Wire CI proto-check; run full test suite and lint.

## Risks And Recovery

- Missing local `protoc`: document in README; CI installs `protobuf-compiler`.
- Generated drift: `make proto-check` fails CI until committed.

## Progress

- [x] Create this plan
- [x] Add `node_control.proto`
- [x] Codegen + committed `gen/` output
- [x] Fixture compile tests
- [x] CI + docs + validation

## Decisions

- 2026-09-16: Place compile helper under `internal/adapters/federated/wire`
  (not domain) to satisfy architecture import boundaries while keeping wire
  mapping next to generated stubs consumers.

## Validation

- `make proto && make proto-check` — passed
- `go test ./internal/adapters/federated/wire/...` — passed
- `go test ./...` — passed
- `make lint` — passed

## Result

Proto v1 implemented per decision 0004. Wire contract lives in
`api/proto/lifecloud/node/v1/node_control.proto`; stubs in `gen/lifecloud/node/v1/`.
Demo fixture compile proof in `internal/adapters/federated/wire/`. gRPC server
runtime remains follow-up work.
