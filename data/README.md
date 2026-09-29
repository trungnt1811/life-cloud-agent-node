# Demo Hospital Data

`federated_mvp/` is a copy of `life-cloud/data/federated_mvp/` (dataset
"federated-data-v2", generator seed `20260910`; see
`federated_mvp/manifest.json`). It is simulated data traced back to a public
Kaggle HPLC screening dataset — **not** records collected at a real hospital.
`federated_mvp/README.md` is the upstream description (Vietnamese) and stays
authoritative for what each file means.

Copied:

- `nodes/{VN_A,VN_B,VN_C}/` — byte-identical to upstream. This is the only
  input a node is meant to see: `raw/results.csv` + `raw/reexport.csv` are what
  `cmd/ingest` loads; `mapping.json` documents the per-site rules the adapters
  in `internal/adapters/ingestion/` implement; `raw/patient_registry.csv` is
  not read by the node today.
- `benchmark/expected_queries.csv` and `benchmark/query_definition.json` — the
  demo cohort and its answer key, used to verify a seeded node.
- Small metadata: `manifest.json`, `node_summary.csv`, `source_profile.csv`,
  `verification_report.json`, `benchmark/*_simulation.json`.

Not copied (~84 MB): the rest of `benchmark/` (per-row answer files) and
`inspection/`. Upstream says those belong to the test harness / offline
inspection and must not be handed to a node pipeline.

Do not edit these files here. Regenerate upstream in `life-cloud` and copy
again. To load them into a node, follow
[`docs/runbooks/seed-demo-hospital-data.md`](../docs/runbooks/seed-demo-hospital-data.md).
