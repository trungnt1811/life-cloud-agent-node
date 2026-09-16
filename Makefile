APP_BIN ?= life-cloud-agent-node
GOLANGCI_LINT_VERSION ?= v1.64.8
TEMPLATE_MODULE := github.com/lifenetwork-ai/go-backend-template
TEMPLATE_REPO_NAME := go-backend-template
TEMPLATE_DB_NAME := go_backend_template

DB_NAME ?= life_cloud_agent_node
DB_USER ?= postgres
DB_PASSWORD ?= postgres
DB_PORT ?= 5432
POSTGRES_REPOSITORY_TEST_IMAGE ?= postgres:15-alpine

.PHONY: build clean run test test-coverage lint swagger swagger-check template-identity-check migrate mockgen mocks dev-up dev-down docker-db-up docker-db-down test-postgres-repositories test-postgres-repositories-fast

build:
	go build -o ./bin/$(APP_BIN) ./cmd/main.go

clean:
	go clean ./...
	@mkdir -p ./bin
	@find ./bin -type f -name '$(APP_BIN)' -delete
	@find . -type f -name 'coverage.out' -delete

run:
	go run ./cmd/main.go

test:
	go test ./...

test-postgres-repositories:
	POSTGRES_REPOSITORY_TEST_IMAGE="$(POSTGRES_REPOSITORY_TEST_IMAGE)" go test ./internal/adapters/repositories/... -count=1

test-postgres-repositories-fast:
	TESTCONTAINERS_RYUK_DISABLED=true POSTGRES_REPOSITORY_TEST_IMAGE="$(POSTGRES_REPOSITORY_TEST_IMAGE)" POSTGRES_REPOSITORY_TEST_REUSE_CONTAINER=true POSTGRES_REPOSITORY_TEST_REUSE_SCOPE=package go test ./internal/adapters/repositories/... -count=1

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --fix

swagger:
	go run github.com/swaggo/swag/cmd/swag@v1.16.3 init -g ./cmd/main.go -d ./ -o ./docs

swagger-check: swagger
	git diff --exit-code -- docs/docs.go docs/swagger.json docs/swagger.yaml

template-identity-check:
	@repo_name=$$(basename "$$(git rev-parse --show-toplevel 2>/dev/null || pwd)"); \
	if [ "$$repo_name" = "$(TEMPLATE_REPO_NAME)" ]; then \
		echo "template identity check skipped for canonical $(TEMPLATE_REPO_NAME) repository"; \
		exit 0; \
	fi; \
	fail=0; \
	if [ "$$(go list -m)" = "$(TEMPLATE_MODULE)" ]; then \
		echo "go.mod still uses template module $(TEMPLATE_MODULE)"; \
		fail=1; \
	fi; \
	if grep -q '^APP_BIN ?= $(TEMPLATE_REPO_NAME)$$' Makefile; then \
		echo "Makefile APP_BIN still uses $(TEMPLATE_REPO_NAME)"; \
		fail=1; \
	fi; \
	if grep -q 'prefix($(TEMPLATE_MODULE))' .golangci.yml; then \
		echo ".golangci.yml gci prefix still uses $(TEMPLATE_MODULE)"; \
		fail=1; \
	fi; \
	if grep -Eq '^(APP_NAME=$(TEMPLATE_REPO_NAME)|DB_NAME=$(TEMPLATE_DB_NAME))$$' .env.example; then \
		echo ".env.example still contains template APP_NAME or DB_NAME"; \
		fail=1; \
	fi; \
	if grep -q '$(TEMPLATE_DB_NAME)' docker-compose.yml; then \
		echo "docker-compose.yml still contains template DB name $(TEMPLATE_DB_NAME)"; \
		fail=1; \
	fi; \
	if [ "$$fail" -ne 0 ]; then \
		echo "rename template identity before product feature work"; \
		exit 1; \
	fi

migrate:
	go run ./cmd/migration/main.go

mocks: mockgen

mockgen:
	go generate ./...

dev-up:
	docker compose up -d db redis

dev-down:
	docker compose down

docker-db-up:
	docker run --name $(APP_BIN)-db \
		-e POSTGRES_DB=$(DB_NAME) \
		-e POSTGRES_USER=$(DB_USER) \
		-e POSTGRES_PASSWORD=$(DB_PASSWORD) \
		-p $(DB_PORT):5432 \
		-d postgres:15-alpine

docker-db-down:
	docker stop $(APP_BIN)-db
	docker rm $(APP_BIN)-db
