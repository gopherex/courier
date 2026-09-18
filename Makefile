SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

# Run against published module dependencies, even inside a parent workspace.
override export GOWORK := off

MAX_MAJOR := 1

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

.PHONY: docs-deps docs-build docs-start test-tooling release
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

## release: interactive tag-driven release (Docker image + @gopherex/courier-sdk)
## WARNING: Option 2 (recreate last tag) force-pushes tags by deleting and
## recreating them on the current HEAD. This is intended for LOCAL development
## use only — do NOT use this in CI/CD pipelines. In production, release tags
## MUST be treated as immutable once published.
.PHONY: release
release:
	@set -eu; \
	cd "$$(git rev-parse --show-toplevel)"; \
	if [ -n "$$(git status --porcelain)" ]; then \
	  echo "Working tree is not clean — commit or stash first:"; \
	  git status --short; \
	  exit 1; \
	fi; \
	cur="$$(git tag -l 'v[0-9]*.[0-9]*.[0-9]*' | sed 's/^v//' | sort -t. -k1,1n -k2,2n -k3,3n | tail -1)"; \
	cur="$${cur:-0.0.0}"; \
	head="$$(git rev-parse --short HEAD)"; \
	echo "Latest release: v$$cur    HEAD: $$head"; \
	echo; \
	echo "  1) bump version"; \
	echo "  2) recreate last tag (v$$cur) on HEAD   [force]"; \
	echo "  3) cancel"; \
	read -r -p "> " action; \
	case "$$action" in \
	1) \
	  MA="$${cur%%.*}"; rest="$${cur#*.}"; MI="$${rest%%.*}"; PA="$${rest#*.}"; \
	  echo; \
	  echo "  1) major  -> v$$((MA+1)).0.0"; \
	  echo "  2) minor  -> v$$MA.$$((MI+1)).0"; \
	  echo "  3) patch  -> v$$MA.$$MI.$$((PA+1))"; \
	  read -r -p "> " comp; \
	  case "$$comp" in \
	    1) MA=$$((MA+1)); MI=0; PA=0 ;; \
	    2) MI=$$((MI+1)); PA=0 ;; \
	    3) PA=$$((PA+1)) ;; \
	    *) echo "Aborted."; exit 0 ;; \
	  esac; \
	  if [ "$$MA" -gt "$(MAX_MAJOR)" ]; then \
	    echo "v$$MA requires semantic import versioning (/v$$MA in the Go module path)."; \
	    echo "Not supported yet — stay on v0/v1."; \
	    exit 1; \
	  fi; \
	  new="$$MA.$$MI.$$PA"; \
	  echo; \
	  echo "Release v$$new — will:"; \
	  echo "  - set @gopherex/courier-sdk version $$new"; \
	  echo "  - update the admin workspace SDK dependency and yarn.lock"; \
	  echo "  - commit 'chore(release): release v$$new'"; \
	  echo "  - create tag v$$new and push HEAD + tag"; \
	  echo "  - CI will publish ghcr.io/gopherex/courier:$$new + latest and @gopherex/courier-sdk@$$new"; \
	  read -r -p "Type 'yes' to proceed: " ok; \
	  [ "$$ok" = "yes" ] || { echo "Aborted."; exit 0; }; \
	  VERSION="$$new" node -e "const fs=require('fs'); const p='sdk/ts/package.json'; const j=JSON.parse(fs.readFileSync(p,'utf8')); j.version=process.env.VERSION; fs.writeFileSync(p, JSON.stringify(j,null,2)+'\n'); const w='web/package.json'; const a=JSON.parse(fs.readFileSync(w,'utf8')); a.dependencies['@gopherex/courier-sdk']=process.env.VERSION; fs.writeFileSync(w, JSON.stringify(a,null,2)+'\n');"; \
	  yarn install >/dev/null; \
	  git add -A; \
	  git diff --cached --quiet || git commit -m "chore(release): release v$$new"; \
	  git tag -a "v$$new" -m "v$$new"; \
	  git push origin HEAD; \
	  git push origin "v$$new"; \
	  echo "Released v$$new."; \
	  ;; \
	2) \
	  if [ "$$cur" = "0.0.0" ] && ! git tag -l 'v0.0.0' | grep -q .; then \
	    echo "No release tag to recreate."; exit 1; \
	  fi; \
	  pkg_ver="$$(node -p 'require("./sdk/ts/package.json").version')"; \
	  if [ "$$pkg_ver" != "$$cur" ]; then \
	    echo "@gopherex/courier-sdk version $$pkg_ver does not match v$$cur."; \
	    echo "Use bump release instead of recreating the tag."; \
	    exit 1; \
	  fi; \
	  echo; \
	  echo "Will DELETE and recreate tag v$$cur on $$head, then force-push."; \
	  read -r -p "Type 'yes' to proceed: " ok; \
	  [ "$$ok" = "yes" ] || { echo "Aborted."; exit 0; }; \
	  remote_tag="$$(git ls-remote --tags origin "refs/tags/v$$cur")"; \
	  if [ -n "$$remote_tag" ]; then git push origin ":refs/tags/v$$cur"; fi; \
	  git tag -d "v$$cur"; \
	  git tag -a "v$$cur" -m "v$$cur"; \
	  git push origin --force "v$$cur"; \
	  echo "Recreated v$$cur on $$head."; \
	  ;; \
	*) \
	  echo "Cancelled."; \
	  ;; \
	esac
