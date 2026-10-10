package contracts_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSharedPhotosWireContracts(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, "../../api/external/sharedphotos.openapi.yaml")

	paths, err := filepath.Glob("../replay/fixtures/synthetic/http/photos-upload-shared-*.json")
	if err != nil || len(paths) != 14 {
		t.Fatal("shared photo contract inventory differs", err)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			for _, exchange := range accountExchanges(t, path) {
				var route string

				switch {
				case strings.HasSuffix(exchange.Request.Path, "/webgetalbumslist"):
					route = "/{accountID}/sharedstreams/webgetalbumslist"
				case strings.HasSuffix(exchange.Request.Path, "/webgetassets"):
					route = "/webgetassets"
				case strings.HasSuffix(exchange.Request.Path, "/webgetassetcount"):
					route = "/webgetassetcount"
				default:
					continue
				}

				operation := document.Paths.Value(route).Post
				validateDriveRequest(t, operation, exchange.Request)
				validateDriveResponse(t, operation, exchange.Response)
			}
		})
	}
}

func TestSharedPhotosRequiredFields(t *testing.T) {
	t.Parallel()

	document := loadDriveDocument(t, "../../api/external/sharedphotos-models.openapi.yaml")
	for _, schema := range []string{"SharedAssetsRequest", "SharedCountRequest", "SharedAlbumsResponse",
		"SharedAlbum", "SharedCountResponse"} {
		if document.Components.Schemas[schema].Value.VisitJSON(map[string]any{}) == nil {
			t.Fatal("required field negative accepted", schema)
		}
	}

	for _, value := range []any{nil, -1, "1", true} {
		if document.Components.Schemas["SharedCountResponse"].Value.VisitJSON(
			map[string]any{"albumassetcount": value}) == nil {
			t.Fatal("malformed count accepted", value)
		}
	}

	if document.Components.Schemas["SharedPluginFields"].Value.VisitJSON(
		map[string]any{"likedByCaller": "true"}) == nil {
		t.Fatal("non-object shared plugin accepted")
	}

	if document.Components.Schemas["SharedAlbumsRequest"].Value.VisitJSON(
		map[string]any{"unexpectedAlbumsKey": true}) == nil {
		t.Fatal("unexpected album request key accepted")
	}
}

func TestSharedPhotosGenerationHasNoDrift(t *testing.T) {
	t.Parallel()

	for _, artifact := range []generationArtifact{
		{Schema: "../../api/external/sharedphotos.openapi.yaml",
			Config: "../../pkg/dependencies/webtransport/sharedphotosapi/config.yaml",
			Output: "../../pkg/dependencies/webtransport/sharedphotosapi/client.gen.go"},
		{Schema: "../../api/external/sharedphotos-models.openapi.yaml",
			Config: "../../pkg/dependencymodels/sharedphotos/config.yaml",
			Output: "../../pkg/dependencymodels/sharedphotos/models.gen.go"},
	} {
		t.Run(artifact.Output, func(t *testing.T) { t.Parallel(); verifyGeneration(t, artifact) })
	}
}
