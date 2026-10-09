package contracts_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/internal/photosuploadapi"
)

const photosUploadSchemaPath = "../../api/external/photos-upload.openapi.yaml"

func TestPhotosUploadGenerationHasNoDrift(t *testing.T) {
	t.Parallel()
	verifyGeneration(t, generationArtifact{Schema: photosUploadSchemaPath,
		Config: "../../internal/photosuploadapi/config.yaml", Output: "../../internal/photosuploadapi/client.gen.go"})
}

func TestPhotosUploadReceiptPreservesUnknownNumber(t *testing.T) {
	t.Parallel()

	var receipt photosuploadapi.PhotosSingleFileUpload

	err := json.Unmarshal([]byte(`{"providerGeneration":9007199254740993}`), &receipt)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}

	var fields map[string]json.RawMessage

	err = json.Unmarshal(encoded, &fields)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(fields["providerGeneration"], []byte("9007199254740993")) {
		t.Fatal("unknown receipt integer lost precision")
	}
}

func TestPhotosUploadWireContracts(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, photosUploadSchemaPath)

	paths, err := filepath.Glob("../replay/fixtures/synthetic/http/photos-upload-*.json")
	if err != nil {
		t.Fatal(err)
	}

	pairs := 0

	for _, path := range paths {
		for _, exchange := range accountExchanges(t, path) {
			item := document.Paths.Value(exchange.Request.Path)
			if item == nil || !strings.HasPrefix(exchange.Request.Path, "/photosupload/") {
				continue
			}

			operation := item.GetOperation(exchange.Request.Method)
			validateFindMyParameters(t, operation, exchange.Request)
			validateDriveRequest(t, operation, exchange.Request)

			pairs++

			if exchange.Response.Status >= 400 || strings.Contains(path, "invalid-json") {
				continue
			}

			if strings.Contains(path, "invalid-payload") {
				validatePhotoCountInvalidReply(t, operation, exchange.Response, path)
			} else {
				validateDriveResponse(t, operation, exchange.Response)
			}
		}
	}

	if pairs == 0 {
		t.Fatal("Photos upload paired contracts were not exercised")
	}
}

func TestPhotosUploadSchemasRejectMalformedPayloads(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, photosUploadSchemaPath)

	cases := map[string]string{
		"PhotosCreateUploadUrlRequest":   `{"zoneName":"PrimarySync","assets":{"file":"three"}}`,
		"PhotosSingleFileUploadResponse": `{"singleFile":{"size":"three"}}`,
		"PhotosPutAssetRequest":          `{"zoneName":"PrimarySync","files":[],"importGroup":"group"}`,
		"PhotosUploadStatusRequest":      `{"uploadJobIds":null}`,
		"PhotosUploadStatusEntries":      `{"job":{"progress":"done"}}`,
		"PhotosPutAssetResults":          `[{"response":{"isRetryable":"yes"}}]`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var value any

			err := json.Unmarshal([]byte(payload), &value)
			if err != nil {
				t.Fatal(err)
			}

			if document.Components.Schemas[name].Value.VisitJSON(value) == nil {
				t.Fatal("malformed upload payload accepted")
			}
		})
	}
}
