---
name: go-clean-architecture-convention
description: Enforce this repository's Go backend clean architecture and coding conventions. Use when implementing or reviewing handlers, usecases, repositories, adapter services, tests, migrations, pagination APIs, or dependency wiring so changes follow requirements/coding_convention.md.
---

# Go Clean Architecture Convention

Always enforce `requirements/coding_convention.md` as the source of truth before writing code.

## Workflow

1. Before product feature work in a derived repository, verify template identity has been renamed with `make template-identity-check`.
2. Identify affected layer first: `delivery`, `usecase`, `repository`, `adapter service`, `di`, `test`, or `migration`.
3. Read the relevant section from `requirements/coding_convention.md`.
4. Apply layer constraints strictly:
   - Keep dependency flow: `Delivery -> Usecase -> Repository`.
   - Keep repository free from DTO imports.
   - Keep business logic out of handlers.
5. Implement with convention guardrails:
   - Move magic values to `constants/`.
   - Enforce pagination defaults and bounds for list APIs.
   - Inject dependencies through constructors and `internal/di`.
   - Wrap API-facing errors with domain custom errors.
   - Log technical failures in usecase/adapter layers.
   - Open and control DB transactions in usecase layer when write chains are involved.
6. Apply testing rules:
   - Use `gomock` generated mocks (no handwritten fake mocks).
   - Use PostgreSQL Testcontainers for repository/transaction adapter tests; do not add alternate SQL fallbacks.
   - Prefer table-driven tests and `require` assertions.
7. For schema/data scripts:
   - Place schema migrations in `internal/adapters/postgres/scripts/`.
   - Place one-off data scripts in `internal/adapters/postgres/manual_scripts/`.
   - Keep scripts idempotent and forward-only.
8. Validate the final change with the checklist in `references/checklist.md`.

## Fast lookup

Use these quick lookups when you need exact wording:

- Architecture and folder rules: `requirements/coding_convention.md` sections `1` and `2`.
- Data boundaries, entities, usecases, repositories: sections `3` through `6`.
- HTTP, config/DI, logging/errors, testing: sections `7` through `10`.

## Output contract for this skill

When this skill is used, always include:

1. Which convention rules were applied.
2. Any intentional deviation and why.
3. Remaining risks or follow-up checks (if any).
