# Go Backend Template

A production-ready Go API template using Clean Architecture principles with Gin, PostgreSQL, Redis/in-memory cache, structured logging, explicit dependency composition, and architecture guards.

## Features

- **Clean Architecture**: Well-structured codebase following Clean Architecture principles
- **RESTful API**: Built with Gin web framework
- **Database**: PostgreSQL with GORM ORM and migrations
- **Caching**: Redis integration with configurable TTL
- **Configuration**: Environment-based configuration with Viper and module-scoped runtime config
- **Logging**: Structured logging with Zap
- **Documentation**: Swagger/OpenAPI integration
- **Testing**: Unit testing setup with testify, gomock, architecture guards, and integration test helpers
- **Docker**: Multi-stage Dockerfile and docker-compose setup
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

### Template Bootstrap

Before writing product code in a repository created from this template, replace the template identity with the real repository identity:

- `go.mod` module path
- `.golangci.yml` `gci` local import prefix
- `APP_BIN` in `Makefile`
- `APP_NAME` and `DB_NAME` in `.env.example`
- database names in `docker-compose.yml`

Then run:

```bash
make template-identity-check
```

### Using Docker Compose (Recommended)

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd go-backend-template
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
make template-identity-check # Verify derived repos no longer use template identity
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
APP_NAME=go-backend-template
APP_PORT=8080
APP_REQUEST_TIMEOUT_MS=30000
ENV=development
LOG_LEVEL=info

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=go_backend_template
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

This boilerplate follows Clean Architecture principles:

- **Delivery**: HTTP-only concerns. Handlers map request DTOs to domain contracts and map usecase output back to response DTOs.
- **Domain**: Contracts, entities, usecases, repository ports, and external service ports. Domain does not import Gin, GORM, HTTP response packages, client SDKs, or adapter models.
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

The boilerplate includes example CRUD endpoints:

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
docker build -t go-backend-template .

# Run container
docker run -p 8080:8080 go-backend-template
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

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Clean Architecture by Robert C. Martin
- Gin Web Framework
- GORM ORM
- And all the amazing Go community packages used in this project
