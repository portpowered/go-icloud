package contracts_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/portpowered/go-icloud/tests/replay"
)

const photoMutationOrigin = "origin"
const photoContentHTTPScheme = "https"

func TestPhotoDownloadWireContracts(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, "../../api/external/photos.openapi.yaml")

	paths, err := filepath.Glob("../replay/fixtures/synthetic/http/photos-download-*.json")
	if err != nil || len(paths) != 25 {
		t.Fatal("photo download contract inventory changed", err)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			validatePhotoDownloadExchanges(t, document, path)
		})
	}
}

func validatePhotoDownloadExchanges(t *testing.T, document *openapi3.T, path string) {
	t.Helper()
	exchanges := accountExchanges(t, path)

	for index, exchange := range exchanges {
		item := document.Paths.Value(exchange.Request.Path)

		if exchange.Request.Method == http.MethodGet {
			if index == 0 || !issuedPhotoURL(t, path, exchanges[index-1], exchange.Request) {
				t.Fatal("photo download URL was not issued for the selected photo rendition")
			}

			item = document.Paths.Value(driveContentPath)
		}

		if item == nil {
			t.Fatal("unbound photo download route")
		}

		operation := item.GetOperation(exchange.Request.Method)
		if operation == nil {
			t.Fatal("unbound photo download method")
		}

		if exchange.Request.Method != http.MethodGet {
			validateFindMyParameters(t, operation, exchange.Request)
		}

		validateDriveRequest(t, operation, exchange.Request)

		if exchange.Response != nil {
			validateDriveResponse(t, operation, exchange.Response)
		}
	}
}

func issuedPhotoURL(t *testing.T, path string, previous replay.Exchange, request replay.Request) bool {
	t.Helper()

	if previous.Response == nil || previous.Response.Status != http.StatusOK {
		return false
	}

	target := photoContractIssuedTarget(t, path, previous)

	parsed, err := url.Parse(target)
	if err != nil || parsed.Scheme != photoContentHTTPScheme || parsed.Host == "" ||
		parsed.Scheme+"://"+parsed.Host != request.Origin || parsed.EscapedPath() != request.Path {
		return false
	}

	observed := url.Values{}
	for _, pair := range request.Query {
		observed.Add(pair[0], pair[1])
	}

	return reflect.DeepEqual(parsed.Query(), observed)
}

func photoContractIssuedTarget(t *testing.T, path string, previous replay.Exchange) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	var row struct {
		Inputs  []string `json:"inputs"`
		Version string   `json:"version"`
	}

	if json.Unmarshal(data, &row) != nil || len(row.Inputs) != 1 {
		t.Fatal("invalid photo download contract input")
	}

	value, err := driveJSONValue(previous.Response.Body)
	if err != nil {
		t.Fatal(err)
	}

	return photoContractResourceURL(value, row.Inputs[0], row.Version)
}

func photoContractResourceURL(value any, photoID, version string) string {
	object, valid := value.(map[string]any)
	if !valid {
		return ""
	}

	records, valid := object["records"].([]any)
	if !valid {
		return ""
	}

	masterID := ""

	for _, item := range records {
		record, _ := item.(map[string]any)
		if record["recordType"] == "CPLAsset" && record["recordName"] == photoID {
			masterID = photoContractMasterID(record, photoID)
		}
	}

	for _, item := range records {
		record, _ := item.(map[string]any)
		if record["recordType"] == "CPLMaster" && record["recordName"] == masterID {
			return photoContractMasterURL(record, version)
		}
	}

	return ""
}

func photoContractMasterID(record map[string]any, fallback string) string {
	fields, _ := record["fields"].(map[string]any)
	field, _ := fields["masterRef"].(map[string]any)
	value, _ := field["value"].(map[string]any)

	id, _ := value["recordName"].(string)
	if id == "" {
		return fallback
	}

	return id
}

func photoContractMasterURL(record map[string]any, version string) string {
	keys := map[string]string{"": "resOriginalRes", "original": "resOriginalRes", "alternative": "resOriginalAltRes",
		"medium": "resJPEGMedRes", "original_video": "resOriginalVidComplRes"}
	fields, _ := record["fields"].(map[string]any)
	field, _ := fields[keys[version]].(map[string]any)
	value, _ := field["value"].(map[string]any)
	target, _ := value["downloadURL"].(string)

	return target
}

func TestPhotoDownloadBindingRejectsChangedSignedTarget(t *testing.T) {
	t.Parallel()

	path := "../replay/fixtures/synthetic/http/photos-download-escaped-signed-url.json"
	exchanges := accountExchanges(t, path)
	previous := exchanges[len(exchanges)-2]

	request := exchanges[len(exchanges)-1].Request

	if !issuedPhotoURL(t, path, previous, request) {
		t.Fatal("valid selected photo resource was not bound")
	}

	for _, mutation := range []string{photoMutationOrigin, mutationPath, "query"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()

			changed := request

			switch mutation {
			case photoMutationOrigin:
				changed.Origin = "https://wrong-assets.example.invalid"
			case mutationPath:
				changed.Path += "/wrong"
			default:
				changed.Query = append(append([]replay.Pair{}, request.Query...), replay.Pair{"x", "3"})
			}

			if issuedPhotoURL(t, path, previous, changed) {
				t.Fatal("changed provider target was accepted")
			}
		})
	}
}
