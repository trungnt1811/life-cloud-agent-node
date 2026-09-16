# Life Cloud Node Agent

The node-side data agent that runs at a single hospital site in the Life
Cloud federated thalassemia registry. Each hospital runs its own instance
against its own local data; no raw patient data leaves the node. See
[docs/product/overview.md](docs/product/overview.md) for the product intent
and [docs/decisions/0001-node-agent-role-and-grpc-channel.md](docs/decisions/0001-node-agent-role-and-grpc-channel.md)
for the accepted architecture. Agents working in this repository should start
from [AGENTS.md](AGENTS.md).

Built on a Clean Architecture Go API foundation with Gin, PostgreSQL,
Redis/in-memory cache, structured logging, explicit dependency composition,
and architecture guards.

## Current Status

Template bootstrap is complete (module, app, and database identity match this
repository). Architecture and the gRPC wire contract / query schema v1 are
decided (`docs/decisions/0001`–`0004`). The `.proto` source and generated Go
stubs live under `api/proto/` and `gen/`; regenerate with `make proto`. No
gRPC server or federated query runtime exists yet — the
`internal/domain/.../example*` CRUD code is still the template's placeholder
domain, not the thalassemia data model. See `docs/plans/` for active and
completed work.

## Features

- **Clean Architecture**: layered codebase separating delivery, domain, and
  adapters
- **REST (Gin)**: node-local HTTP surface for health checks, admin, and
  Swagger/OpenAPI docs
- **gRPC (planned)**: the federated query channel between the central
  coordinator and this node agent; not yet implemented (decision 0001)
- **Database**: PostgreSQL with GORM ORM and migrations
- **Caching**: Redis integration with configurable TTL
- **Configuration**: environment-based configuration with Viper and
  module-scoped runtime config
- **Logging**: structured logging with Zap
- **Documentation**: Swagger/OpenAPI integration
- **Testing**: unit testing with testify, gomock, architecture guards, and
  integration test helpers
- **Docker**: multi-stage Dockerfile and docker-compose setup
- **Linting**: golangci-lint configuration

## Project Structure

```
├── cmd/                    # Application entrypoints
│   ├── app/                # Main application
│   ├── main.go             # Application entry point
│   └── migration/          # Database migration tool
├── conf/                   # Environment loading only
├── constants/              # Application constants
├── internal/               # Private application code
│   ├── adapters/           # Outbound adapters: repositories, services, postgres scripts
│   ├── delivery/           # Inbound HTTP layer: router, routes, handlers, middleware, response
│   ├── domain/             # Contracts, entities, repository ports, usecases
│   ├── di/                 # Dependency composition
│   ├── platform/           # Cross-cutting platform APIs, such as logger
│   ├── runtimeconfig/      # Module-scoped config projection
│   ├── server/             # HTTP server lifecycle
│   ├── testsupport/        # Test helpers and architecture assertions
│   └── workers/            # Background workers
├── infrastructures/        # Infrastructure clients/drivers reused by adapters
└── docs/                   # API documentation
```

## Prerequisites

- Go 1.24+
- PostgreSQL 15+
- Redis 7+
- Docker & Docker Compose. Docker is required for repository/transaction tests because they run against isolated PostgreSQL containers.

## Quick Start

### Using Docker Compose (Recommended)

1. **Clone the repository**
   ```bash
   git clone git@github.com:lifenetwork-ai/life-cloud-agent-node.git
   cd life-cloud-agent-node
   ```

2. **Start all services**
   ```bash
   docker-compose up -d
   ```

3. **Access the API**
   - API: http://localhost:8080
   - Swagger Documentation: http://localhost:8080/swagger/index.html

### Manual Setup

1. **Install dependencies**
   ```bash
   go mod download
   ```

2. **Set up environment variables**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

3. **Run database migrations**
   ```bash
   make migrate
   ```

4. **Start the application**
   ```bash
   make run
   ```

## Development

### Available Make Commands

```bash
make build          # Build the application
make run            # Run the application
make test           # Run tests
make test-coverage  # Run tests with coverage
make lint           # Run linter
make swagger        # Generate Swagger documentation
make swagger-check  # Verify generated Swagger/OpenAPI docs are current
make proto          # Generate gRPC/protobuf Go stubs
make proto-check    # Verify generated protobuf stubs are current
make template-identity-check # Verify the repository no longer uses the upstream template identity
make migrate        # Run database migrations
make clean          # Clean build artifacts
make mockgen        # Regenerate gomock mocks through go generate
make dev-up         # Start local Postgres and Redis
make dev-down       # Stop local dependencies
```

### Development Tools Setup

1. **Install mockgen for testing**
   ```bash
   go install go.uber.org/mock/mockgen@latest
   ```

2. **Generate mocks**
   ```bash
   make mockgen
   ```

3. **Protobuf codegen uses a pinned toolchain**
   ```bash
   make proto          # downloads pinned protoc + plugins into tools/bin/
   make proto-check    # regenerates and fails if gen/ drifts
   ```
   Do not rely on system `protoc` (Homebrew/apt versions differ and break CI).

### Testing

**Run all tests:**
```bash
go test ./...
```

Repository and transaction tests use PostgreSQL Testcontainers with the real migration scripts from `internal/adapters/postgres/scripts/`. There is no alternate in-memory SQL fallback, so keep Docker running before executing the full test suite.

**Run tests with coverage:**
```bash
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

**Run specific test:**
```bash
go test -v ./internal/domain/usecases/
```

## Configuration

The application uses environment variables for configuration. Key variables:

```env
# Application
APP_NAME=life-cloud-agent-node
APP_PORT=8080
APP_REQUEST_TIMEOUT_MS=30000
ENV=development
LOG_LEVEL=info

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=life_cloud_agent_node
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5
DB_CONN_MAX_LIFETIME_IN_MINUTE=60

# Redis
REDIS_ADDRESS=localhost:6379
REDIS_PASSWORD=
REDIS_TTL=10m

# Cache
CACHE_TYPE=redis
```

## Architecture

This repository follows Clean Architecture principles:

- **Delivery**: HTTP-only concerns today. Handlers map request DTOs to domain
  contracts and map usecase output back to response DTOs. A gRPC delivery
  path is planned for the federated query channel (decision 0001) and will
  live alongside this layer, not replace it.
- **Domain**: Contracts, entities, usecases, repository ports, and external
  service ports. Domain does not import Gin, GORM, HTTP response packages,
  client SDKs, gRPC-generated code, or adapter models.
- **Adapters**: Persistence and external service implementations. Repositories map GORM models to domain records/entities; service adapters call HTTP/RPC/KMS/third-party clients behind domain ports.
- **Infrastructure**: Reusable clients/drivers such as cache and rate limiter implementations.
- **DI/Runtime Config**: Startup composes dependencies and projects environment config into module-scoped config structs.

### Key Components

- **Entities**: Encapsulated domain objects with constructors, behavior, and record/snapshot mapping.
- **Contracts**: Domain-facing input/output structs used by usecases.
- **DTOs**: HTTP transport objects only.
- **Repositories**: Domain interfaces plus adapter implementations.
- **External Service Ports**: Interfaces under `internal/domain/usecases/services`; concrete HTTP/RPC/client calls live under `internal/adapters/services`.
- **Handlers**: Thin HTTP request/response mapping.
- **Transaction Manager**: Domain transaction port with GORM adapter implementation and post-commit hooks.
- **Mapping Tests**: Reflection-assisted mapper field coverage via `internal/testsupport/mappingtest`.
- **Architecture Guards**: Tests that block layer leaks, handwritten test doubles, and oversized files.

### Required PR Checks

Every PR should fill `.github/pull_request_template.md` and run:

```bash
make mockgen
make template-identity-check
make swagger-check
make test-postgres-repositories
go test ./...
make lint
git diff --check
```

## API Endpoints

The codebase still ships the template's placeholder CRUD endpoints, not yet
replaced by the federated thalassemia domain:

```
GET    /api/v1/examples     # List examples
POST   /api/v1/examples     # Create example
GET    /api/v1/examples/:id # Get example by ID
PUT    /api/v1/examples/:id # Update example
DELETE /api/v1/examples/:id # Delete example
```

## Docker

### Build and run with Docker

```bash
# Build image
docker build -t life-cloud-agent-node .

# Run container
docker run -p 8080:8080 life-cloud-agent-node
```

### Using docker-compose

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

## Acknowledgments

- Clean Architecture by Robert C. Martin
- Gin Web Framework
- GORM ORM
- Bootstrapped from the internal `go-backend-template` and
  [repository-harness](https://github.com/hoangnb24/repository-harness)
