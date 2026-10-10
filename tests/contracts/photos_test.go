package contracts_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestPhotoAlbumWireContracts(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, "../../api/external/photos.openapi.yaml")

	paths, err := filepath.Glob("../replay/fixtures/synthetic/http/photos-albums-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 42 {
		t.Fatal("photo album contract inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			validatePhotoAlbumExchanges(t, document, path)
		})
	}
}

func validatePhotoAlbumExchanges(t *testing.T, document *openapi3.T, path string) {
	t.Helper()

	exchanges := accountExchanges(t, path)
	for index, exchange := range exchanges {
		item := document.Paths.Value(exchange.Request.Path)
		if item == nil {
			t.Fatal("unbound photo album route")
		}

		operation := item.GetOperation(exchange.Request.Method)
		if operation == nil {
			t.Fatal("unbound photo album method")
		}

		validateFindMyParameters(t, operation, exchange.Request)
		validateDriveRequest(t, operation, exchange.Request)

		invalid := index == len(exchanges)-1 && photoQueryInvalidReply(path)
		if invalid {
			validatePhotoCountInvalidReply(t, operation, exchange.Response, path)
		} else {
			validateDriveResponse(t, operation, exchange.Response)
		}
	}
}

func TestPhotosInitializationWireContracts(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, "../../api/external/photos.openapi.yaml")

	paths, err := filepath.Glob("../replay/fixtures/synthetic/http/photos-index-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 27 {
		t.Fatal("photo initialization contract inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			validatePhotosInitialization(t, document, path)
		})
	}
}

func TestPhotosGenerationHasNoDrift(t *testing.T) {
	t.Parallel()
	verifyGeneration(t, generationArtifact{Schema: "../../api/external/photos.openapi.yaml",
		Config: "../../pkg/dependencies/webtransport/photosapi/config.yaml",
		Output: "../../pkg/dependencies/webtransport/photosapi/client.gen.go"})
}

func validatePhotosInitialization(t *testing.T, document *openapi3.T, path string) {
	t.Helper()

	exchanges := accountExchanges(t, path)
	if len(exchanges) != 1 {
		t.Fatal("photo initialization exchange count changed")
	}

	exchange := exchanges[0]

	item := document.Paths.Value(exchange.Request.Path)
	if item == nil {
		t.Fatal("unbound photo initialization route")
	}

	operation := item.GetOperation(exchange.Request.Method)
	if operation == nil {
		t.Fatal("unbound photo initialization method")
	}

	validateFindMyParameters(t, operation, exchange.Request)
	validateDriveRequest(t, operation, exchange.Request)

	invalid := filepath.Base(path) == "photos-index-records-null.json" ||
		filepath.Base(path) == "photos-index-invalid-records-shape.json" ||
		filepath.Base(path) == "photos-index-later-malformed.json"
	if invalid {
		contract := driveResponseContract(operation, exchange.Response.Status)

		value, err := driveJSONValue(exchange.Response.Body)
		if err != nil {
			t.Fatal(err)
		}

		if contract.Value.Content["application/json"].Schema.Value.VisitJSON(value) == nil {
			t.Fatal("Source-invalid photo response accepted by schema")
		}
	} else {
		validateDriveResponse(t, operation, exchange.Response)
	}
}

func TestPhotoAssetsWireContracts(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, "../../api/external/photos.openapi.yaml")

	paths, err := filepath.Glob("../replay/fixtures/synthetic/http/photos-assets-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 110 {
		t.Fatal("photo asset contract inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) { t.Parallel(); validatePhotoAlbumExchanges(t, document, path) })
	}
}

func photoQueryInvalidReply(path string) bool {
	for _, suffix := range []string{"records-null.json", "later-record-malformed.json",
		"later-page-invalid.json", "invalid-last-record.json",
		"metadata-int64-fraction-invalid.json", "metadata-int64-overflow-invalid.json"} {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}

	return false
}

func TestPhotosMutationsGenerationHasNoDrift(t *testing.T) {
	t.Parallel()
	verifyGeneration(t, generationArtifact{Schema: "../../api/external/photos-mutations-models.openapi.yaml",
		Config: "../../pkg/dependencymodels/photosmutations/config.yaml",
		Output: "../../pkg/dependencymodels/photosmutations/models.gen.go"})
}
