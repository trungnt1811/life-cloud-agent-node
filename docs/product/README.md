# Product Docs

This directory contains current consumer-product behavior derived from real
accepted intent. Harness deliberately ships no fake product domains.

When a user provides a product specification, derive smaller living documents
here instead of keeping one growing specification as the operating manual. Name
files after actual product domains, such as `overview.md`, `billing.md`,
`permissions.md`, or `api-conventions.md`.

## Current Product Contract

- `overview.md` — node-agent role in the federated thalassemia registry.
- `federated-query-flow.md` — DFD / sequence / data-flow dictionary for the
  gRPC channel and local stores.
- `query-field-dictionary.md` — schema v1 global `field_code` set and demo
  compile hint (pairs with decision 0004).
- `node-agent-connection-guide.md` — technical spec for whoever implements
  the control center's `NodeControl` gRPC server: connection lifecycle,
  message contracts, validation order, retry/resume semantics, and known v1
  limitations.

## Update Rule

When behavior changes:

1. Update the affected product document when the expected behavior changed.
2. Update the active execution plan when complex work uses one.
3. Add a lasting decision only when future work must inherit a consequential
   product, architecture, data, security, compatibility, or validation choice.
4. Add or update executable proof that exercises the behavior.

Bounded changes do not require a parallel lifecycle record.
