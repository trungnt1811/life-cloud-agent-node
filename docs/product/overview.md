# Product Overview

## What This Repository Is

`life-cloud-agent-node` is the node-side data agent that runs at a single
hospital site in the Life Cloud federated thalassemia registry. Each hospital
runs its own instance against its own local data; no raw patient data leaves
the node.

Related project context: `life-cloud` (sibling repository) holds the research
notes, data model, and federated-query demo data (`docs/research/thalassemia-patient-data.md`,
`data/federated_mvp/`) that this agent's data model and query semantics are
expected to follow.

## Role In The Federated Architecture

- A central coordinator sends distributed queries to every node agent and
  aggregates the results. The coordinator does not receive raw patient rows.
- Each node agent owns its local store (patient, diagnosis, lab, transfusion,
  chelation records ingested from that hospital's own systems) and answers
  queries against it locally.
- This repository builds one node agent. The coordinator is a separate,
  not-yet-built service.

## Protocol Split

- gRPC is the federated query channel: coordinator-to-node calls that submit a
  query and return an aggregated/query result. This is the externally
  observable contract other services depend on.
- REST (Gin, already scaffolded by the template) remains for
  node-local concerns: health checks, admin, and Swagger/OpenAPI docs. It is
  not the federated interface.

See `docs/decisions/` for the accepted decisions and rationale:
`0001` (node role and gRPC channel), `0002` (query contract, four-layer
validation, execution model), `0003` (update and release process). See
`docs/product/federated-query-flow.md` for the DFD 0/1/2, sequence
diagrams, and data flow dictionary those decisions were reviewed against.

## Current Status

Template bootstrap is complete (see `docs/plans/completed/`). The
architecture and query/update contracts are decided (`docs/decisions/`), but
no gRPC service, `.proto` contract, or federated query domain model is
implemented yet. The example CRUD code is still the template's placeholder
domain.
