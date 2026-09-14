# Go Clean Architecture & Coding Conventions

This document is the source of truth for the template structure and coding
conventions. When adding new features, keep boundaries explicit, make code easy
to test, and let the compiler catch misuse early.

## 0. Repository Bootstrap

Before writing product feature code in a repository created from this template, replace the template identity with the real repository identity:

- `go.mod` module path;
- `.golangci.yml` `gci` local import prefix;
- `APP_BIN` in `Makefile`;
- `APP_NAME` and `DB_NAME` in `.env.example`;
- database names in `docker-compose.yml`;
- README examples that should show the product name instead of the template name.

Run `make template-identity-check` after renaming. The check intentionally skips the canonical `go-backend-template` repository, but it must pass in derived service repositories.

## 1. Dependency Rule

Dependencies point inward:

```text
delivery/http -> domain/usecases -> domain ports/contracts/entities
adapters -> domain ports/contracts/entities
infrastructures -> reusable technical clients
```

Domain code must not import Gin, GORM, HTTP response packages, adapter models,
`conf`, `di`, `server`, `workers`, or `infrastructures`.

## 2. Folder Layout

```text
├── cmd/
│   ├── app/
│   └── migration/
├── conf/                           # Load env with Viper; no business logic
├── constants/                      # Shared constants/defaults
├── docs/                           # Swagger/OpenAPI output
├── infrastructures/                # Technical clients/drivers reusable by adapters
│   └── mocks/                      # gomock-generated mocks for infrastructure ports
├── internal/
│   ├── adapters/
│   │   ├── postgres/               # Migration scripts
│   │   ├── repositories/           # GORM models, mappers, repository impl
│   │   └── services/               # External API/service adapters
│   ├── delivery/
│   │   ├── dto/                    # HTTP request/response DTOs only
│   │   └── http/
│   │       ├── handlers/
│   │       ├── middleware/
│   │       ├── response/
│   │       ├── route/
│   │       └── router/
│   ├── di/                         # Manual dependency composition
│   ├── domain/
│   │   ├── contracts/              # Usecase input/output contracts
│   │   ├── entities/               # Encapsulated domain entities + records
│   │   ├── repositories/           # Repository and transaction ports
│   │   ├── types/
│   │   └── usecases/
│   │       ├── errors/
│   │       ├── interfaces/
│   │       └── services/           # External service ports
│   ├── mocks/                      # gomock-generated mocks
│   ├── platform/                   # Cross-cutting interfaces, e.g. logger
│   ├── runtimeconfig/              # Module-scoped config projection
│   ├── server/                     # HTTP server lifecycle/timeouts
│   ├── testsupport/                # Shared test helpers and arch guards
│   │   ├── archtest/               # Architecture/import/file-size/test-double guards
│   │   └── mappingtest/            # Reflection helpers for mapper field coverage
│   └── workers/
├── requirements/
└── tests/
```

Do not create generic `pkg`, `utils`, or `packages` directories. Put code in the layer that owns the behavior.

## 3. Data Boundaries

- HTTP DTOs live in `internal/delivery/dto` and must not be imported by domain or repositories.
- Domain usecases accept and return structs from `internal/domain/contracts`.
- Repositories accept domain entities/primitives and return domain entities.
- Adapter repositories own persistence models under `internal/adapters/repositories/models`.
- Large entities should expose `Record()` / `NewXFromRecord()` snapshots for persistence mapping.
- DTO response mapping should read compact entity views or usecase output contracts, not persistence models.

Never reuse a GORM model as a domain entity just to reduce mapper code. Manual boundary mapping is intentional; add mapper tests when a schema changes.

Expected mapping flow:

```text
HTTP request JSON
  -> internal/delivery/dto request
  -> internal/domain/contracts input
  -> internal/domain/entities entity
  -> internal/domain/entities record
  -> internal/adapters/repositories/models GORM model

GORM model
  -> entity record
  -> entity
  -> usecase output contract
  -> HTTP response DTO
  -> JSON response
```

Mapper tests should use `internal/testsupport/mappingtest` for field coverage. For same-named structs, prefer:

```go
mappingtest.AssertSameNamedFieldCoverage(t, entities.ExampleRecord{}, models.Example{})
```

For intentional field renames or gaps, use `AssertFieldCoverage`, `IgnoreSource`, and `IgnoreTarget`.

## 4. Entities

Entities should protect state with constructors and behavior methods. Prefer private fields plus getters/views for read access.

Allowed:

```go
example := entities.NewExampleEntity(uuid.New(), input.Name, input.Description, time.Now().UTC())
record := example.Record()
```

Avoid public data bags:

```go
example.Status = "fake"
```

For simple CRUD templates, keep behavior minimal but still avoid leaking persistence tags into domain entities.

The included example resource demonstrates the full stack: contracts, entity,
mapper, repository, cache decorator, transaction manager, DTO, handler, and
route. Product services should not cargo-cult every layer for trivial resources:
add cache decorators, transaction wrappers, and extra mapper types only when the
resource actually needs that boundary or behavior.

## 5. Usecases

- Usecases contain application/business flow only.
- Usecases do not import Gin, GORM, HTTP response packages, adapter models, `conf`, or DI packages.
- Usecases should depend on repository interfaces from `internal/domain/repositories` and service ports from `internal/domain/usecases/services`.
- Prefer command/input objects when a function needs several request-scoped values.
- Do not put request metadata into `context.Context`; reserve context for cancellation, deadlines, and tracing.
- Multi-write transactions should be exposed through a domain port or repository method. Do not pass raw `*gorm.DB` into domain interfaces.

### External Service Ports

- External dependencies such as HTTP APIs, RPC providers, KMS clients, queues,
  or third-party SDKs are modeled as service ports under
  `internal/domain/usecases/services/<name>_service.go`.
- Usecases call only the port interface. They must not construct clients,
  serialize transport payloads, parse HTTP/RPC status codes, or know auth
  details for the external system.
- Concrete external calls live under `internal/adapters/services`. The adapter
  owns client setup, request/response mapping, transport errors, retryable error
  classification, and any provider-specific metadata.
- Wire service adapters through `internal/di` only when a usecase actually needs
  that external dependency.

## 6. Repositories

- Repositories are adapter implementations of domain repository ports.
- Repositories may use GORM/SQL and adapter models.
- Repositories must not import delivery DTOs.
- Cache decorators should wrap repository interfaces when cache behavior is cross-cutting.
- Keep model/entity mappers explicit and covered by tests for non-trivial records.
- Repository adapters that support transactions may expose `WithTx(*gorm.DB)` internally, but this method must not appear in domain repository interfaces.

## 6.1 Transaction Manager

- Domain transaction boundary lives behind `repositories.TransactionManager`.
- Usecases should call `txManager.WithinTx(ctx, func(repos repositories.TxRepositories) error { ... })` for multi-write flows.
- `TxRepositories` exposes repositories already bound to the same transaction.
- GORM transaction implementation lives under `internal/adapters/repositories/transaction`.
- `txhooks` lives under `internal/adapters/repositories/txhooks` and is adapter-only. Use it for post-commit hooks such as cache writes.
- `txhooks.WithTransaction` must rollback on error/panic and only run deferred hooks after commit succeeds.

## 7. Delivery HTTP

- Handlers parse and validate transport input, convert DTO -> domain contract, call usecase, then convert output -> DTO.
- Handlers should not contain business rules.
- Pass `c.Request.Context()` to usecases.
- Use request timeout middleware/server timeouts rather than relying only on downstream clients.
- Successful business API responses should return explicit DTO structs directly
  from handlers. Do not hide success payloads behind a generic `data any`
  envelope unless the endpoint contract explicitly requires that wrapper.
- Error responses must use `internal/delivery/http/response.Error`.
- Swagger annotations must match the actual response body: use `{object}
  dto.XxxDTO` for success responses and `{object} dto.ErrorDTOResponse` for
  error responses. Health probes may keep their specialized payload.

## 8. Config and DI

- `conf` loads environment values and is the source of truth for runtime
  defaults. `.env.example`, `docker-compose.yml`, and Makefile defaults are
  operator/developer samples and must be kept aligned when runtime defaults
  change.
- `internal/runtimeconfig` projects env config into scoped module config structs.
- `internal/di` composes repositories, services, usecases, workers, and adapters.
- Background workers implement `internal/workers/types.Worker` with `Name()` and
  `Start(ctx)`. Keep job-specific payload contracts inside the feature package
  that owns the worker.
- Feature modules should expose their own config struct/normalization as they grow. Startup should compose, not own business defaults.

## 9. Logging and Errors

- Use structured logger fields, not printf-style logger APIs.
- Log technical failures at adapter/usecase boundaries with enough context to debug.
- API-facing errors must be normalized through domain error types.
- Field-level error metadata must use typed `domain/usecases/errors.ErrorDetail`
  values, not arbitrary `[]interface{}` payloads.
- Preserve root causes with wrapping/`Unwrap()` while keeping public response messages safe.

## 10. Testing and Mocks

- Unit tests use `go.uber.org/mock/gomock` generated mocks.
- Do not hand-write fake/stub structs for interfaces unless a test helper is a pure data builder and does not implement a production port.
- Add `//go:generate mockgen ...` next to interfaces and regenerate with `make mockgen`.
- Infrastructure interface mocks must be generated into `infrastructures/mocks`, not nested package-specific mock folders.
- Repository and transaction adapter tests must run on PostgreSQL Testcontainers with real migrations from `internal/adapters/postgres/scripts/`.
- Do not add alternate in-memory SQL fallbacks for repository behavior; green tests must exercise the production SQL dialect.
- CI and local release checks must run `make test-postgres-repositories` before
  merging repository, transaction, mapper, migration, or SQL-heavy changes.
- For faster local loops, use `make test-postgres-repositories-fast`; it reuses one warm PostgreSQL container, creates a migrated template database keyed by migration fingerprint, and resets package databases with `TRUNCATE ... CASCADE`.
- `POSTGRES_REPOSITORY_TEST_SKIP_CONTAINER=true` is allowed only for local feedback when Docker is unavailable. CI must keep repository tests on PostgreSQL.
- Prefer table-driven tests and `require` assertions.
- Architecture guards must stay green:
  - domain production code cannot import outer layers;
  - usecases cannot import runtime/config/DI/framework/persistence packages;
  - adapters cannot import delivery packages;
  - delivery cannot import persistence adapters;
  - handwritten `Fake`/`Stub`/`Mock`/`Spy` test doubles are blocked outside generated mock folders;
  - production files should stay reviewable;
  - large test files should be split by behavior.

## 11. Pagination

- List endpoints use `page` default `1` and `page_size` default `20`.
- Enforce `page >= 1` and `page_size <= constants.DefaultMaxPageSize`.
- Convert to repository pagination with `limit = page_size`, `offset = (page - 1) * page_size`.
- Paginated responses include `items`, `total_count`, `page`, and `page_size`.

## 12. Migrations and Scripts

- Schema migrations live in `internal/adapters/postgres/scripts/`.
- Never edit already-applied migrations; add forward-only migration files.
- Manual data scripts live in `internal/adapters/postgres/manual_scripts/`.
- Scripts must be idempotent using `IF NOT EXISTS`, `ON CONFLICT`, or explicit existence checks.

Before submitting code, run:

```bash
make mockgen
make template-identity-check
make swagger-check
go test ./...
make lint
git diff --check
```

## 13. AI/PR Checklist

Every AI-generated or human PR must fill `.github/pull_request_template.md` and explicitly state:

- touched layer(s);
- whether template identity rename is done or not applicable;
- boundary impact;
- API/user impact;
- whether mocks were regenerated;
- tests/lint commands run;
- migration/config impact;
- backward compatibility risk;
- logging/error handling impact.

CI enforces generated-code cleanliness by running `make mockgen` and `git diff --exit-code`. CI also runs `make swagger-check` so OpenAPI docs cannot drift from handler annotations. CI checks that any `make lint --fix` changes are already committed.
