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

# Schema-owned account, Drive, Find My and Reminders wire models; Photos/auth contracts remain pending.
generate-api:
	$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config pkg/dependencymodels/account/config.yaml api/external/account-models.openapi.yaml
	$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config internal/accountapi/config.yaml api/external/account.openapi.yaml
	$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config pkg/dependencymodels/drive/config.yaml api/external/drive-models.openapi.yaml
	$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config internal/driveapi/config.yaml api/external/drive.openapi.yaml
	$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config internal/drivecontentapi/config.yaml api/external/drive-content.openapi.yaml
	$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config pkg/dependencymodels/findmy/config.yaml api/external/findmy-models.openapi.yaml
	$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config internal/findmyapi/config.yaml api/external/findmy.openapi.yaml
	$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config pkg/dependencymodels/cloudkit/config.yaml api/external/cloudkit-models.openapi.yaml
	$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config internal/remindersapi/config.yaml api/external/reminders.openapi.yaml
	$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config pkg/icloud/config.yaml api/client-models.openapi.yaml
	$(GO) run ./tools/apiconstants

# LIB-07: measure replay, unit and combined separately, including internal transport.
sdk-coverage:
	$(GO) test -race '-coverpkg=./pkg/icloud,./internal/...' '-coverprofile=coverage-replay.out' ./tests/replay
	$(GO) run ./tools/coverage -profile coverage-replay.out -min 80
	$(GO) test -race '-coverpkg=./pkg/icloud,./internal/...' '-coverprofile=coverage-unit.out' ./pkg/icloud ./internal/accountapi ./internal/webtransport
	$(GO) run ./tools/coverage -profile coverage-unit.out -min 0
	$(GO) test -race '-coverpkg=./pkg/icloud,./internal/...' '-coverprofile=coverage-combined.out' ./tests/replay ./pkg/icloud ./internal/accountapi ./internal/webtransport
	$(GO) run ./tools/coverage -profile coverage-combined.out -min 80
