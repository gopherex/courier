SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

# Run against published module dependencies, even inside a parent workspace.
override export GOWORK := off

GOLANGCI_VERSION := v2.12.2
GOLANGCI_DIR := $(CURDIR)/bin/tools/golangci-lint/$(GOLANGCI_VERSION)
GOLANGCI := $(GOLANGCI_DIR)/golangci-lint

.PHONY: help tools config-check fmt fmt-check tidy tidy-check vet lint test test-race check clean

## help: list available commands
help:
	@awk '/^## / {sub(/^## /, ""); split($$0, entry, ": "); printf "  %-16s %s\n", entry[1], entry[2]}' $(MAKEFILE_LIST)

## tools: install the pinned linter and formatters into bin/
tools: $(GOLANGCI)

$(GOLANGCI):
	GOBIN="$(GOLANGCI_DIR)" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)

## config-check: validate the linter configuration
config-check: tools
	"$(GOLANGCI)" config verify --config .golangci.yaml

## fmt: format Go source files
fmt: tools
	"$(GOLANGCI)" fmt ./...

## fmt-check: report formatting differences without modifying files
fmt-check: tools
	"$(GOLANGCI)" fmt --diff ./...

## tidy: synchronize module dependencies
tidy:
	go mod tidy

## tidy-check: check module dependencies without modifying files
tidy-check:
	go mod tidy -diff

## vet: run Go static analysis
vet:
	go vet ./...

## lint: run all configured linters over the whole module
lint: tools
	"$(GOLANGCI)" run --build-tags=integration ./...

## test: run Go tests
test:
	go test ./...

## test-race: run uncached tests with the race detector (requires CGO and a C compiler)
test-race:
	CGO_ENABLED=1 go test -race -count=1 ./...

## check: run the same checks as CI
check:
	$(MAKE) config-check
	$(MAKE) tidy-check
	$(MAKE) fmt-check
	$(MAKE) vet
	$(MAKE) lint
	$(MAKE) test
	$(MAKE) test-race
	$(MAKE) generate-check
	$(MAKE) build-web
	$(MAKE) test-tooling

## clean: remove local tools and build artifacts
clean:
	rm -rf -- bin

OGEN_VERSION := v1.20.3
SQLD_VERSION := v1.1.3
SQLD_DIR := $(CURDIR)/bin/tools/sqld/$(SQLD_VERSION)
SQLD := $(SQLD_DIR)/sqld

.PHONY: generate generate-go generate-db migration outbox-schema build run
$(SQLD):
	GOBIN="$(SQLD_DIR)" go install github.com/gopherex/sqld/cmd/sqld@$(SQLD_VERSION)
	GOBIN="$(SQLD_DIR)" go install github.com/gopherex/sqld/cmd/sqld-gen-go@$(SQLD_VERSION)

## generate-go: generate the public Go server and client
generate-go:
	go run github.com/ogen-go/ogen/cmd/ogen@$(OGEN_VERSION) --target pkg/api --package api --clean openapi/openapi.yaml

outbox-schema:
	mkdir -p .build
	cat "$$(go list -m -f '{{.Dir}}' github.com/gopherex/pg-outbox)"/migrations/0*.sql > .build/outbox.sql

## generate-db: generate PostgreSQL models and queries
generate-db: $(SQLD) outbox-schema
	"$(SQLD)" generate -c sqld.yaml

## generate: regenerate all contracts
generate: generate-go generate-db generate-ts

## migration: generate a schema-diff migration (NAME required; Docker required)
migration: $(SQLD) outbox-schema
	test -n "$(NAME)"
	"$(SQLD)" migrate generate "$(NAME)" -c sqld.yaml

.PHONY: node-deps generate-ts generate-check build-web build-sdk integration dev-env dev-up migrate run
## node-deps: install JavaScript dependencies from the lockfile
node-deps:
	yarn install --frozen-lockfile --non-interactive

## generate-ts: generate the TypeScript SDK
generate-ts:
	yarn generate

## generate-check: verify all generated files, including additions and deletions
generate-check:
	python3 scripts/check-generated.py

## build-sdk: build the distributable TypeScript SDK
build-sdk:
	yarn workspace @gopherex/courier-sdk build

## build-web: typecheck and build the administrator UI
build-web: build-sdk
	yarn workspace @gopherex/courier-admin build

## build: build the service binary and admin assets
build: build-web
	go build -trimpath -o bin/courier ./cmd/courier

## integration: run PostgreSQL and SMTP integration tests with the race detector (Docker required)
integration:
	CGO_ENABLED=1 go test -race -tags integration -count=1 ./internal/service

## dev-env: create a fresh local .env without overwriting one
dev-env:
	python3 scripts/dev-env.py

## dev-up: start local PostgreSQL and Mailpit
dev-up:
	docker compose up -d --wait

## migrate: apply migrations using the local environment
migrate:
	set -a; source .env; set +a; go run ./cmd/courier migrate

## run: run Courier using the local environment
run:
	set -a; source .env; set +a; go run ./cmd/courier

.PHONY: browser-test
## browser-test: verify admin UI against a running local service and Mailpit
browser-test:
	mkdir -p .build
	node scripts/browser-test.mjs

.PHONY: docs-deps docs-build docs-start test-tooling release release-plan
## docs-deps: install the documentation toolchain from its lockfile
docs-deps:
	yarn --cwd website install --frozen-lockfile --non-interactive

## docs-build: build documentation with strict link checking
docs-build:
	cp openapi/openapi.yaml website/static/openapi.yaml
	yarn --cwd website build

## docs-start: serve the documentation locally
docs-start:
	cp openapi/openapi.yaml website/static/openapi.yaml
	yarn --cwd website start --host 127.0.0.1

## test-tooling: verify release version and publication preparation
test-tooling:
	python3 -m unittest discover -s scripts -p 'test_*.py'

## release-plan: show the next release without modifying files or pushing
release-plan:
	python3 scripts/release.py --plan $(if $(VERSION),--version $(VERSION),)

## release: interactively prepare and push an immutable release tag
release:
	python3 scripts/release.py $(if $(VERSION),--version $(VERSION),)
