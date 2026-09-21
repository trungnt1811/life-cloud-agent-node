# Decisions

Decision records preserve lasting product, architecture, data ownership,
security, compatibility, and validation choices that future work must inherit.

Use `docs/templates/decision.md`. Task-local implementation choices remain in
the active execution plan and do not require a separate decision.

An installed consumer begins with no fabricated decisions. Add local decision
documents here as real choices are accepted, then index them in this file.

## Index

- [0001 Node Agent Role And gRPC Federated Channel](0001-node-agent-role-and-grpc-channel.md) — this repo is the per-hospital node agent; gRPC carries the federated query channel, REST stays node-local.
- [0002 Federated Query Contract And Execution Model](0002-federated-query-contract-and-execution.md) — async job submission, structured (never raw SQL) queries, four-layer node-side validation, two-layer field whitelisting, automatic execution, aggregate-only suppressed output.
- [0003 Node Update And Release Process](0003-node-update-and-release-process.md) — control center only advises over gRPC; updates are applied locally via `docker compose pull && up -d`; nodes may run different versions at once.
- [0004 gRPC Wire Contract And Query Schema v1](0004-grpc-wire-contract-schema-v1.md) — `NodeControl.Connect` bidi envelopes, schema v1 `QueryTask`/`QueryResult` (including panel + specimen policy), session/advisory fields, and demo-fixture compile mapping; field codes remain strings backed by `docs/product/query-field-dictionary.md`.
- [0005 Job Checkpoint Granularity And Resume Semantics](0005-job-checkpoint-granularity.md) — resumable unit is a patient-ID chunk (default 5000), one checkpoint per chunk, suppression only on the final total, criteria-hash guards `job_id` reuse, finished jobs kept 7 days.
