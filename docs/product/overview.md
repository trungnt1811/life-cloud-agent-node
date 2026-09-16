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

See `docs/decisions/0001-node-agent-role-and-grpc-channel.md` for the accepted
decision and rationale.

## Current Status

Template bootstrap in progress. No federated query domain model or gRPC
service is implemented yet; see `docs/plans/active/` for the current plan.
