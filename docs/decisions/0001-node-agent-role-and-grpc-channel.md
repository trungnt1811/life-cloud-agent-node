# 0001 Node Agent Role And gRPC Federated Channel

Date: 2026-09-16

## Status

Accepted

## Context

This repository started from the generic Go Backend Template with no product
identity. The Life Cloud project needs software for its federated thalassemia
registry: hospital data stays local to each site, and a central coordinator
queries every site and aggregates results (see the sibling `life-cloud`
repository's `docs/superpowers/specs/2026-09-10-federated-demo-data-design.md`
and `data/federated_mvp/README.md` for the data shape this will eventually
serve). It was ambiguous whether this repository is that per-hospital node
agent, the central coordinator, or an unrelated LLM-agent service, and whether
its external interface is REST, gRPC, or both.

## Decision

- This repository (`life-cloud-agent-node`) is the **node-side data agent**
  that runs at one hospital site. The coordinator is a separate, not-yet-built
  service.
- The federated query interface between the coordinator and this node agent is
  **gRPC**. This is the contract other services depend on and the one that
  carries query requests and results across the node boundary.
- The existing REST layer (Gin, from the template) is **retained but scoped
  down** to node-local concerns: health checks, admin, and Swagger/OpenAPI
  docs. It is not used for federated query traffic.

## Alternatives Considered

1. REST-only federated interface (keep the template's existing HTTP layer as
   the sole contract). Rejected: the user explicitly asked to add gRPC as the
   inter-node connection.
2. gRPC-only, removing REST entirely. Rejected: REST/Gin, health checks, and
   Swagger tooling are already scaffolded and used for node-local operability;
   removing them was not requested and has no stated benefit yet.
3. Expose the same functionality over both REST and gRPC as dual public APIs.
   Rejected: the two protocols would need to stay behaviorally identical with
   no current consumer for a public REST federated API, doubling surface area
   for no requested use case.

## Consequences

Positive:

- Clear separation: gRPC is the strongly-typed, service-to-service federated
  contract; REST stays for humans and local operability.
- The template's existing HTTP scaffolding (router, middleware, Swagger) is
  not thrown away.

Tradeoffs:

- The repository now needs a second server (gRPC) running alongside the
  existing HTTP server, with its own DI wiring, and its own test/lint
  coverage.
- The concrete wire contract is recorded in decision 0004; `.proto`
  source and generated stubs are checked in (`api/proto/`, `gen/`).

## Follow-Up

- Concrete gRPC service contract (proto definitions, RPC methods, query
  shape): addressed by decision 0004; `.proto` + codegen landed — gRPC
  server/client runtime remains follow-up.
- Track bootstrap and gRPC scaffolding work in plans under `docs/plans/`.
