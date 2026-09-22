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

PROTOC_VERSION ?= 29.3
PROTOC_GEN_GO_VERSION ?= v1.36.5
PROTOC_GEN_GO_GRPC_VERSION ?= v1.5.1
PROTO_FILES := api/proto/lifecloud/node/v1/node_control.proto
TOOLS_DIR := $(CURDIR)/tools
TOOLS_BIN := $(TOOLS_DIR)/bin
PROTOC := $(TOOLS_BIN)/protoc
PROTOC_INCLUDE := $(TOOLS_DIR)/include

.PHONY: build clean run test test-coverage lint swagger swagger-check proto proto-tools proto-check template-identity-check migrate ingest mockgen mocks dev-up dev-down docker-db-up docker-db-down test-postgres-repositories test-postgres-repositories-fast test-integration

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

test-integration:
	POSTGRES_REPOSITORY_TEST_IMAGE="$(POSTGRES_REPOSITORY_TEST_IMAGE)" go test ./tests/integration/... -count=1

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --fix

swagger:
	go run github.com/swaggo/swag/cmd/swag@v1.16.3 init -g ./cmd/main.go -d ./ -o ./docs

swagger-check: swagger
	git diff --exit-code -- docs/docs.go docs/swagger.json docs/swagger.yaml

proto-tools:
	@mkdir -p "$(TOOLS_BIN)" "$(PROTOC_INCLUDE)"
	@if ! "$(PROTOC)" --version 2>/dev/null | grep -Fq "$(PROTOC_VERSION)"; then \
		os=$$(uname -s); arch=$$(uname -m); \
		case "$$os-$$arch" in \
			Linux-x86_64)   platform=linux-x86_64 ;; \
			Linux-aarch64)  platform=linux-aarch_64 ;; \
			Darwin-x86_64)  platform=osx-x86_64 ;; \
			Darwin-arm64)   platform=osx-aarch_64 ;; \
			*) echo "unsupported platform for pinned protoc: $$os $$arch"; exit 1 ;; \
		esac; \
		tmp=$$(mktemp -d); \
		url="https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-$${platform}.zip"; \
		echo "downloading pinned protoc $(PROTOC_VERSION) ($${platform})"; \
		curl -fsSL "$$url" -o "$$tmp/protoc.zip"; \
		unzip -qo "$$tmp/protoc.zip" -d "$$tmp/protoc"; \
		install -m 755 "$$tmp/protoc/bin/protoc" "$(PROTOC)"; \
		rm -rf "$(PROTOC_INCLUDE)"; \
		mkdir -p "$(PROTOC_INCLUDE)"; \
		cp -R "$$tmp/protoc/include/." "$(PROTOC_INCLUDE)/"; \
		rm -rf "$$tmp"; \
	fi
	@GOBIN="$(TOOLS_BIN)" go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	@GOBIN="$(TOOLS_BIN)" go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)

proto: proto-tools
	@mkdir -p gen
	PATH="$(TOOLS_BIN):$$PATH" "$(PROTOC)" \
	  --proto_path=api/proto \
	  --proto_path="$(PROTOC_INCLUDE)" \
	  --go_out=gen --go_opt=paths=source_relative \
	  --go-grpc_out=gen --go-grpc_opt=paths=source_relative \
	  $(PROTO_FILES)

proto-check: proto
	git diff --exit-code -- gen/

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

ingest:
	go run ./cmd/ingest -profile $(PROFILE) $(FILES)

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
