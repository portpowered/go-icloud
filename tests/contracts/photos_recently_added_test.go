package contracts_test

import (
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/portpowered/go-icloud/tests/replay"
	"testing"
)

func TestRecentlyAddedPhotosWireContracts(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, "../../api/external/photos.openapi.yaml")

	paths, err := filepath.Glob("../replay/fixtures/synthetic/http/photos-recently-added-*.json")
	if err != nil || len(paths) != 21 {
		t.Fatal("recently added contract inventory changed", err)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			validateRecentlyAddedPhotoExchanges(t, document, path)
		})
	}
}

func validateRecentlyAddedPhotoExchanges(t *testing.T, document *openapi3.T, path string) {
	t.Helper()

	for index, exchange := range accountExchanges(t, path) {
		item := document.Paths.Value(exchange.Request.Path)
		if item == nil {
			t.Fatal("unbound Recently Added route")
		}

		operation := item.GetOperation(exchange.Request.Method)
		if operation == nil {
			t.Fatal("unbound Recently Added method")
		}

		validateFindMyParameters(t, operation, exchange.Request)
		validateDriveRequest(t, operation, exchange.Request)

		if index == 2 && recentlyAddedInvalidSharedReply(path) {
			validateRecentlyAddedInvalidSharedReply(t, operation, exchange.Response, path)
		} else {
			validateDriveResponse(t, operation, exchange.Response)
		}
	}
}

func recentlyAddedInvalidSharedReply(path string) bool {
	switch filepath.Base(path) {
	case "photos-recently-added-shared-invalid-zone.json", "photos-recently-added-shared-null-zones.json",
		"photos-recently-added-shared-object-zones.json", "photos-recently-added-shared-array-envelope.json",
		"photos-recently-added-shared-invalid-json.json":
		return true
	default:
		return false
	}
}

func validateRecentlyAddedInvalidSharedReply(t *testing.T, operation *openapi3.Operation,
	response *replay.Response, path string,
) {
	t.Helper()

	value, err := driveJSONValue(response.Body)

	if filepath.Base(path) == "photos-recently-added-shared-invalid-json.json" {
		var syntax *json.SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatal("invalid shared reply lost JSON syntax failure", err)
		}

		return
	}

	if err != nil {
		t.Fatal(err)
	}

	contract := driveResponseContract(operation, response.Status)
	if contract.Value.Content["application/json"].Schema.Value.VisitJSON(value) == nil {
		t.Fatal("invalid shared zone reply was accepted by its contract")
	}
}
