GO ?= go
GOLANGCI_LINT ?= $(if $(wildcard .tools/golangci-lint.exe),.tools/golangci-lint.exe,golangci-lint)
ifeq ($(OS),Windows_NT)
PYTHON ?= $(CURDIR)/.venv/Scripts/python.exe
else
PYTHON ?= .venv/bin/python
endif

.DEFAULT_GOAL := check
.PHONY: check lint build test reference-coverage endpoint-coverage generate-api

# Reference capture bootstrap; no public Go iCloud client is implemented yet.
check: lint build test endpoint-coverage

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
