package contracts_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestPhotoCountWireContracts(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, "../../api/external/photos.openapi.yaml")

	paths, err := filepath.Glob("../replay/fixtures/synthetic/http/photos-count-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 49 {
		t.Fatal("photo count contract inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			validatePhotoCountPairs(t, document, path)
		})
	}
}

func validatePhotoCountPairs(t *testing.T, document *openapi3.T, path string) {
	t.Helper()

	exchanges := accountExchanges(t, path)
	for index, exchange := range exchanges {
		item := document.Paths.Value(exchange.Request.Path)
		if item == nil {
			t.Fatal("unbound photo count route")
		}

		operation := item.GetOperation(exchange.Request.Method)
		if operation == nil {
			t.Fatal("unbound photo count method")
		}

		validateFindMyParameters(t, operation, exchange.Request)
		validateDriveRequest(t, operation, exchange.Request)

		if index == len(exchanges)-1 && photoCountInvalidSchema(path) {
			contract := driveResponseContract(operation, exchange.Response.Status)

			value, err := driveJSONValue(exchange.Response.Body)
			if err != nil {
				t.Fatal(err)
			}

			if contract.Value.Content["application/json"].Schema.Value.VisitJSON(value) == nil {
				t.Fatal("Source-invalid photo count accepted by schema")
			}
		} else {
			validateDriveResponse(t, operation, exchange.Response)
		}
	}
}

func photoCountInvalidSchema(path string) bool {
	for _, suffix := range []string{"batch-null", "records-null", "fields-missing", "itemcount-missing",
		"value-missing", "later-record-invalid", "later-batch-invalid", "fraction", "null-value"} {
		if strings.HasSuffix(path, suffix+".json") {
			return true
		}
	}

	return false
}
