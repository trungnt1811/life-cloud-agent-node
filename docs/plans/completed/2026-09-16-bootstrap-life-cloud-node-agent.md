# Execution Plan: Bootstrap life-cloud-agent-node Identity

Date: 2026-09-16

## Status

Active

## Outcome

The repository stops being an anonymous Go Backend Template clone and becomes
identifiable as the Life Cloud node agent: module path, binary name, app name,
and database names match the real repository, `make template-identity-check`
passes, and the build/test/lint suite still passes after the rename.

## Context

- `docs/decisions/0001-node-agent-role-and-grpc-channel.md`: this repo is the
  per-hospital node agent; gRPC will later carry the federated query channel,
  REST stays for node-local concerns.
- `docs/product/overview.md`: current product intent.
- README.md "Template Bootstrap" section: the template requires identity
  rename before product feature work begins.
- `Makefile`'s `template-identity-check` target enumerates every place the
  template identity (`github.com/lifenetwork-ai/go-backend-template`,
  `go-backend-template`, `go_backend_template`) must be removed from:
  `go.mod`, `.golangci.yml` gci prefix, `Makefile` `APP_BIN`, `.env.example`
  `APP_NAME`/`DB_NAME`, `docker-compose.yml` DB names, plus every Go file that
  imports the old module path (61 files at plan creation time).

## Scope

In scope:

- Rename Go module path to `github.com/lifenetwork-ai/life-cloud-agent-node`
  and update every import across the codebase.
- Update `.golangci.yml` gci local prefix, `Makefile` `APP_BIN`,
  `.env.example` `APP_NAME`/`DB_NAME`, `docker-compose.yml` DB name and
  Postgres DB name to `life-cloud-agent-node` / `life_cloud_agent_node`.
- Verify with `make template-identity-check`, `go build ./...`, `go vet
  ./...`, and the repository's Go test suite.

Out of scope:

- Any gRPC service, proto definitions, or federated query domain model
  (tracked as follow-up in decision 0001; needs its own contract design).
- Any change to the example CRUD domain (`internal/domain/.../example*`) or
  to actual Life Cloud data ingestion. Renaming identity does not imply
  building product behavior yet.

## Approach

1. Update non-Go identity files: `go.mod` module line, `.golangci.yml` gci
   prefix, `Makefile` `APP_BIN`, `.env.example`, `docker-compose.yml`.
2. Rewrite the old module path to the new one across all `.go` files.
3. Run `go build ./...` and `go vet ./...` to confirm the rename is complete
   and consistent.
4. Run `make template-identity-check`.
5. Run `go test ./...` (skip Postgres-container repository tests only if
   Docker is unavailable in this environment; disclose if skipped).
6. Run `make lint` if `golangci-lint` is available; disclose if skipped.

## Risks And Recovery

- Risk: a mechanical rename misses an import or a hardcoded string, breaking
  the build. Mitigation: `go build ./...` after the rewrite catches missed
  imports immediately.
- Risk: renaming touches every Go file, an unusually large diff for review.
  Mitigation: the change is purely a search-and-replace of the module path
  plus a handful of identity constants; no logic changes are bundled in.
- Recovery: the repository is git-tracked with a clean history at
  `6c1a79d`; a bad rename can be reverted with `git checkout -- .` before
  commit, or a follow-up commit after.

## Progress

- [x] Update `go.mod`, `.golangci.yml`, `Makefile`, `.env.example`,
      `docker-compose.yml`.
- [x] Rewrite module path across all Go source files.
- [x] `go build ./...` passes.
- [x] `make template-identity-check` passes.
- [x] `go test ./...` run and result recorded.
- [x] `make lint` run and result recorded (or gap disclosed).

## Decisions

- 2026-09-16: New module path is `github.com/lifenetwork-ai/life-cloud-agent-node`,
  `APP_NAME`/`APP_BIN` is `life-cloud-agent-node`, `DB_NAME` is
  `life_cloud_agent_node`. Derived directly from the existing GitHub remote
  (`lifenetwork-ai/life-cloud-agent-node`); not a materially open choice.
- 2026-09-16: Also rewrote hardcoded template-name defaults not covered by
  `template-identity-check` (`conf/env.go` `defaultAppName`/`defaultDBName`,
  `infrastructures/caching/cache_repository.go` fallback app name, and
  `*-test` fixture names in `tests/integration/setup_test.go`,
  `internal/adapters/repositories/testsupport/postgres_container.go`,
  `internal/adapters/repositories/example_repository_test.go`,
  `infrastructures/caching/cache_repository_test.go`,
  `infrastructures/rate_limiter/fixed_window_rate_limiter_test.go`) for
  consistency. Left `Makefile`'s `TEMPLATE_MODULE`/`TEMPLATE_REPO_NAME`/
  `TEMPLATE_DB_NAME` constants untouched: they are the canonical-template
  markers the check itself compares against, not this repo's identity.
  Left `README.md`'s narrative template references untouched: out of the
  declared scope (mechanical identity rename, not a docs rewrite);
  flagged as follow-up below.

## Validation

- Focused proof: `go build ./...` and `go vet ./...` both pass (exit 0; only
  pre-existing cgo warnings from `github.com/shoenig/go-m1cpu`, unrelated to
  this change).
- Integration or end-to-end proof: `go test ./...` — all packages pass,
  including Postgres/Testcontainers repository and transaction tests (Docker
  was available in this environment).
- Repository-required checks:
  - `make template-identity-check`: pass.
  - `make lint` (golangci-lint v1.64.8 via `go run`, includes `gci` import
    reordering): pass, no remaining diff from the fix pass.
  - `make swagger-check`: pass, regenerated `docs/docs.go`/`swagger.json`/
    `swagger.yaml` are unchanged (still describe the unrelated example CRUD
    domain).
  - `make mockgen` (after installing `go.uber.org/mock/mockgen@latest`,
    absent from this environment by default): regenerated mocks match the
    manually rewritten import paths exactly, confirming the mocks are
    current with the new module path.
  - `git diff --exit-code` for docs/mocks was not run as a standalone gate
    here because the whole rename is one uncommitted change set; it applies
    once this work is committed, per README's PR checklist.

## Result

Complete. The repository now identifies as `life-cloud-agent-node`
end-to-end: module path, binary name, app name, database name, and every
Go import updated; template-identity-check, the full test suite, lint,
swagger generation, and mock generation all pass against the new identity.

Limitations / follow-up:

- `README.md` still narrates the generic "Go Backend Template" (title,
  clone instructions, example `docker build -t go-backend-template`, sample
  `.env` values) — not rewritten in this pass; a separate documentation pass
  should describe `life-cloud-agent-node` as the Life Cloud node agent.
- No gRPC service, proto definitions, or federated query domain model exist
  yet (see `docs/decisions/0001-node-agent-role-and-grpc-channel.md`
  follow-up) — the example CRUD domain is still the only implemented
  feature, unchanged by this identity rename.
