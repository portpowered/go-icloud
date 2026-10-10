package contracts_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestPhotosChangeAndContainerWireContracts(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, "../../api/external/photos.openapi.yaml")
	paths, err := filepath.Glob("../replay/fixtures/synthetic/http/photos-*changes*.json")
	if err != nil {
		t.Fatal(err)
	}
	lookups, err := filepath.Glob("../replay/fixtures/synthetic/http/photos-container-shared-lookup-*.json")
	if err != nil {
		t.Fatal(err)
	}

	paths = append(paths, lookups...)
	if len(paths) != 25 {
		t.Fatalf("changes/container fixture denominator changed: %d", len(paths))
	}

	for _, path := range paths {
		t.Run(
			filepath.Base(path),
			func(t *testing.T) { t.Parallel(); validatePhotosChangesContract(t, document, path) },
		)
	}
}

func validatePhotosChangesContract(t *testing.T, document *openapi3.T, path string) {
	t.Helper()

	exchanges := accountExchanges(t, path)

	for index, exchange := range exchanges {
		item := document.Paths.Value(exchange.Request.Path)
		if item == nil {
			t.Fatal("unbound Photos changes/container route")
		}
		operation := item.GetOperation(exchange.Request.Method)
		validateFindMyParameters(t, operation, exchange.Request)
		validateDriveRequest(t, operation, exchange.Request)
		invalid := index == len(exchanges)-1 && strings.Contains(path, "-schema-error")
		if invalid {
			validatePhotoCountInvalidReply(t, operation, exchange.Response, path)
		} else {
			validateDriveResponse(t, operation, exchange.Response)
		}
		if strings.HasSuffix(exchange.Request.Path, "/changes/zone") {
			checkPhotosChangeRequestNegatives(t, operation, exchange.Request.Body)
		}
	}
}

func checkPhotosChangeRequestNegatives(t *testing.T, operation *openapi3.Operation, body replay.Entity) {
	t.Helper()
	value, err := driveJSONValue(body)
	if err != nil {
		t.Fatal(err)
	}
	schema := operation.RequestBody.Value.Content["application/json"].Schema.Value
	fields, objectPresent := value.(map[string]any)
	if !objectPresent {
		t.Fatal("changes request object missing")
	}
	zones, zonesPresent := fields["zones"].([]any)
	if !zonesPresent || len(zones) != 1 {
		t.Fatal("changes request zone missing")
	}
	zone, zonePresent := zones[0].(map[string]any)
	if !zonePresent {
		t.Fatal("changes zone object missing")
	}
	zone["reverse"] = true
	if schema.VisitJSON(value) == nil {
		t.Fatal("reverse changes accepted")
	}
	zone["reverse"] = false
	fields["zones"] = []any{zone, zone}
	if schema.VisitJSON(value) == nil {
		t.Fatal("multiple zones accepted")
	}
}
