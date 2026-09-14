# Convention Checklist

Use this checklist before submitting any Go backend change.

## Repository bootstrap

- Derived service repositories have replaced `go-backend-template` with the real repository/module/app/database identity.
- `make template-identity-check` passes, or the change is intentionally in the canonical template repository.

## Architecture and layering

- Dependency direction stays `delivery -> usecase -> repository`.
- Handler only maps request/response and delegates logic to usecase.
- Usecase contains business logic and does not depend on HTTP framework or DB ORM details.
- Repository uses entities/primitives only and does not import delivery DTO packages.
- Domain code does not import Gin, GORM, adapter models, `conf`, `di`, `server`, `workers`, or `infrastructures`.
- Adapters do not import delivery packages.
- Delivery does not import persistence adapter packages.

## Data and contracts

- DTOs are used for HTTP transport only.
- Usecases accept/return domain contracts, not delivery DTOs.
- Persistence models stay in adapter repositories; entities expose records/snapshots for mapping.
- Mapping between DTO and domain contract happens at the HTTP boundary.
- Non-trivial mapper tests use `internal/testsupport/mappingtest` for field coverage.
- New code is placed in the expected folder per clean architecture layout.

## Constants and pagination

- No hardcoded business string or magic number in business logic.
- Shared constants/defaults are defined in `constants/`.
- List APIs enforce `page` default `1`, `page_size` default `20`, and max `page_size` `100`.
- Pagination conversion to `limit` and `offset` is correct.
- Paginated responses include `items`, `total`, `page`, `page_size`.

## DI, error, and transaction

- New modules expose constructor functions and are composed via `internal/di`.
- No direct `new()` or inline `&Repo{}` construction inside usecase logic.
- Technical errors are logged at usecase/adapter layer.
- API responses return wrapped domain errors (no leaking raw internal error messages).
- Multi-write DB flows use explicit transaction ports/repository methods; no raw ORM details leak into domain interfaces.

## Testing and mocks

- Unit tests use gomock-generated mocks only.
- No handwritten fake mocks implementing interfaces manually.
- No test type names use `Fake`, `Stub`, `Mock`, or `Spy` outside generated mock folders.
- Repository/transaction adapter tests use PostgreSQL Testcontainers with real migrations, not alternate in-memory SQL fallbacks.
- Tests are table-driven where applicable.
- Assertions use `github.com/stretchr/testify/require`.

## PR and CI

- `.github/pull_request_template.md` is filled for PRs.
- `make mockgen` leaves `git diff --exit-code` clean.
- `make template-identity-check` passes for derived service repositories.
- `make swagger-check` leaves generated OpenAPI docs clean.
- `make lint` leaves `git diff --exit-code` clean.
- `git diff --check` passes.

## Migrations and scripts

- Schema migrations go to `internal/adapters/postgres/scripts/` with ordered filenames.
- Existing applied migration files are not edited (forward-only).
- One-off/manual data scripts go to `internal/adapters/postgres/manual_scripts/`.
- Scripts are idempotent (`IF NOT EXISTS`, `ON CONFLICT DO NOTHING`, existence checks).

## AI and external providers

- SDK/provider-specific code stays in adapters layer.
- Usecase interacts through interfaces and neutral types.
- Retry/refund/fail-safe logic is explicit in usecase flow for AI-related operations.
- Async tasks are offloaded for non-user-blocking side effects.
