GO ?= go
GOLANGCI_LINT ?= $(if $(wildcard .tools/golangci-lint.exe),.tools/golangci-lint.exe,golangci-lint)
ifeq ($(OS),Windows_NT)
PYTHON ?= $(CURDIR)/.venv/Scripts/python.exe
else
PYTHON ?= .venv/bin/python
endif

.DEFAULT_GOAL := check
.PHONY: check lint build test reference-coverage endpoint-coverage generate-api sdk-coverage

# Selected iCloud SDK migration and reference-capture verification.
check: lint build test endpoint-coverage sdk-coverage

lint:
	$(GO) vet ./...
	$(GOLANGCI_LINT) run --timeout=5m ./...
	"$(PYTHON)" -m ruff check tools/reference
	"$(PYTHON)" -m ruff format --check tools/reference

build:
	$(GO) build ./...
	"$(PYTHON)" tools/reference/icloud.py --help

test:
	$(GO) test -race ./...
	"$(PYTHON)" -m unittest discover -s tools/reference -p "test_*.py" -v

# Account-specific reference measurement; never use private captures in CI.
reference-coverage:
	"$(PYTHON)" tools/reference/measure.py

# Diagnostic route occurrences; completeness/schema/socket gates remain open.
endpoint-coverage:
	$(GO) run ./tools/endpointcoverage -summary

# Schema-owned account wire models; additional service contracts remain pending.
generate-api:
	$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config pkg/dependencymodels/account/config.yaml api/external/account-models.openapi.yaml
	$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config internal/accountapi/config.yaml api/external/account.openapi.yaml
	$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config pkg/icloud/config.yaml api/client-models.openapi.yaml
	$(GO) run ./tools/apiconstants

# LIB-07: measure replay, unit and combined separately, including internal transport.
sdk-coverage:
	$(GO) test -race '-coverpkg=./pkg/icloud,./internal/...' '-coverprofile=coverage-replay.out' ./tests/replay
	$(GO) run ./tools/coverage -profile coverage-replay.out -min 80
	$(GO) test -race '-coverpkg=./pkg/icloud,./internal/...' '-coverprofile=coverage-unit.out' ./pkg/icloud ./internal/accountapi
	$(GO) run ./tools/coverage -profile coverage-unit.out -min 0
	$(GO) test -race '-coverpkg=./pkg/icloud,./internal/...' '-coverprofile=coverage-combined.out' ./tests/replay ./pkg/icloud ./internal/accountapi
	$(GO) run ./tools/coverage -profile coverage-combined.out -min 80
