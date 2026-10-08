GO ?= go
GOLANGCI_LINT ?= $(if $(wildcard .tools/golangci-lint.exe),.tools/golangci-lint.exe,golangci-lint)
ifeq ($(OS),Windows_NT)
PYTHON ?= $(CURDIR)/.venv/Scripts/python.exe
else
PYTHON ?= .venv/bin/python
endif

.DEFAULT_GOAL := check
.PHONY: check lint build test reference-coverage

# Reference capture bootstrap; no public Go iCloud client is implemented yet.
check: lint build test

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
