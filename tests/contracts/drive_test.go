package contracts_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/portpowered/go-icloud/tests/replay"
)

const (
	driveSchemaPath             = "../../api/external/drive.openapi.yaml"
	driveContentSchemaPath      = "../../api/external/drive-content.openapi.yaml"
	driveModelsPath             = "../../api/external/drive-models.openapi.yaml"
	driveContentPath            = "/{contentPath}"
	driveContractBinaryEncoding = "base64"
	mutationMethod              = "method"
	mutationPath                = "path"
)

var (
	errDriveBinding  = errors.New("drive exchange has no verified contract binding")
	errContractMedia = errors.New("response has no media contract")
)

type driveDocuments struct {
	Routes  *openapi3.T
	Content *openapi3.T
	Models  *openapi3.T
}

func loadDriveDocument(t *testing.T, path string) *openapi3.T {
	t.Helper()

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	document, err := loader.LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}

	err = document.Validate(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	return document
}

func loadDriveDocuments(t *testing.T) driveDocuments {
	t.Helper()

	return driveDocuments{Routes: loadDriveDocument(t, driveSchemaPath),
		Content: loadDriveDocument(t, driveContentSchemaPath), Models: loadDriveDocument(t, driveModelsPath)}
}

func TestPortableDriveExchangesMatchContracts(t *testing.T) {
	t.Parallel()

	documents := loadDriveDocuments(t)

	paths, err := filepath.Glob("../replay/fixtures/synthetic/http/drive-*.json")
	if err != nil {
		t.Fatal(err)
	}

	pairs := 0
	operations := make(map[string]int)

	for _, path := range paths {
		exchanges := accountExchanges(t, path)
		for index, exchange := range exchanges {
			operation, err := bindDriveOperation(documents, exchanges, index)
			if err != nil {
				t.Fatalf("%s exchange %d: %v", path, index, err)
			}

			validateDriveRequest(t, operation, exchange.Request)
			validateDriveResponse(t, operation, exchange.Response)

			operations[operation.OperationID]++
			pairs++
		}
	}

	if len(paths) != 79 || pairs != 139 || len(operations) != 13 {
		t.Fatalf("Drive inventory changed: scenarios=%d pairs=%d operations=%d", len(paths), pairs, len(operations))
	}
}

func bindDriveOperation(documents driveDocuments, exchanges []replay.Exchange, index int) (*openapi3.Operation, error) {
	request := exchanges[index].Request
	path := request.Path

	zonePath := regexp.MustCompile(`^/ws/[^/]+/(download/by_id|upload/web|update/documents)$`)
	if zonePath.MatchString(path) {
		parts := strings.Split(path, "/")
		parts[2] = "{zone}"
		path = strings.Join(parts, "/")
	}

	if item := documents.Routes.Paths.Value(path); item != nil {
		if operation := item.GetOperation(request.Method); operation != nil {
			return operation, nil
		}

		return nil, errDriveBinding
	}

	if index == 0 || !issuedDriveURL(exchanges[index-1], request) {
		return nil, errDriveBinding
	}

	operation := documents.Content.Paths.Value(driveContentPath).GetOperation(request.Method)
	if operation == nil {
		return nil, errDriveBinding
	}

	return operation, nil
}

func issuedDriveURL(previous replay.Exchange, request replay.Request) bool {
	if previous.Response == nil || !driveIssuerStatus(previous.Response.Status, previous.Request.Method) ||
		previous.Request.Method != request.Method {
		return false
	}

	value, err := driveJSONValue(previous.Response.Body)
	if err != nil {
		return false
	}

	target := issuedDriveTarget(previous.Request.Path, request.Method, value)
	parsed, err := url.Parse(target)

	return err == nil && parsed.Scheme == "https" && parsed.Host != "" &&
		parsed.Scheme+"://"+parsed.Host == request.Origin && parsed.EscapedPath() == request.Path
}

func issuedDriveTarget(path, method string, value any) string {
	switch method {
	case http.MethodGet:
		return driveDownloadTarget(path, value)
	case http.MethodPost:
		return driveUploadTarget(path, value)
	default:
		return ""
	}
}

func driveDownloadTarget(path string, value any) string {
	if !regexp.MustCompile(`^/ws/[^/]+/download/by_id$`).MatchString(path) {
		return ""
	}

	object, valid := value.(map[string]any)
	if !valid {
		return ""
	}

	target := driveTokenURL(object, "data_token")
	if target == "" {
		target = driveTokenURL(object, "package_token")
	}

	return target
}

func driveUploadTarget(path string, value any) string {
	if !regexp.MustCompile(`^/ws/[^/]+/upload/web$`).MatchString(path) {
		return ""
	}

	array, valid := value.([]any)
	if !valid || len(array) == 0 {
		return ""
	}

	object, valid := array[0].(map[string]any)
	if !valid {
		return ""
	}

	target, _ := object["url"].(string)

	return target
}

func driveTokenURL(object map[string]any, key string) string {
	token, ok := object[key].(map[string]any)
	if !ok {
		return ""
	}

	value, _ := token["url"].(string)

	return value
}

func validateDriveRequest(t *testing.T, operation *openapi3.Operation, request replay.Request) {
	t.Helper()

	if operation.RequestBody == nil {
		return
	}

	mediaType := driveHeaderMedia(t, request.Headers)

	media := operation.RequestBody.Value.Content[mediaType]
	if media == nil || media.Schema == nil {
		t.Fatalf("%s request has no media contract: %s", operation.OperationID, mediaType)
	}

	var value any

	if request.Body.Encoding == "multipart" {
		value = driveMultipartValue(t, request.Body)
	} else {
		var err error

		value, err = driveJSONValue(request.Body)
		if err != nil {
			t.Fatal(err)
		}
	}

	err := media.Schema.Value.VisitJSON(value)
	if err != nil {
		t.Fatalf("%s request: %v", operation.OperationID, err)
	}
}

func driveMultipartValue(t *testing.T, entity replay.Entity) map[string]any {
	t.Helper()

	value := make(map[string]any)

	for _, part := range entity.Parts {
		for _, header := range part.Headers {
			if strings.EqualFold(header[0], "Content-Disposition") {
				_, parameters, err := mime.ParseMediaType(header[1])
				if err != nil || parameters["name"] == "" {
					t.Fatal("multipart field has no valid name")
				}

				value[parameters["name"]] = "" // Binary bytes/framing are checked by paired multipart replay.
			}
		}
	}

	return value
}

func validateDriveResponse(t *testing.T, operation *openapi3.Operation, response *replay.Response) {
	t.Helper()

	if response == nil {
		t.Fatal("Drive exchange lost its response")
	}

	declared := driveResponseContract(operation, response.Status)
	if declared == nil {
		t.Fatal("Drive status has no response contract")
	}

	media, err := contractResponseMedia(declared.Value.Content, response.Headers)
	if err != nil {
		t.Fatal(err)
	}

	if media == nil || media.Schema == nil {
		t.Fatal("Drive response media has no contract")
	}

	if media.Schema.Value.Format == binaryComponent {
		return
	}

	value, err := driveJSONValue(response.Body)
	if err != nil {
		t.Fatal(err)
	}

	err = media.Schema.Value.VisitJSON(value)
	if err != nil {
		t.Fatalf("%s response: %v", operation.OperationID, err)
	}
}

func contractResponseMedia(content openapi3.Content, headers []replay.Pair) (*openapi3.MediaType, error) {
	for _, header := range headers {
		if !strings.EqualFold(header[0], "Content-Type") {
			continue
		}

		value, _, err := mime.ParseMediaType(header[1])
		if err != nil {
			return nil, fmt.Errorf("response media type: %w", err)
		}

		if media := content[value]; media != nil {
			return media, nil
		}

		break
	}

	if fallback := content["*/*"]; fallback != nil {
		return fallback, nil
	}

	return nil, errContractMedia
}

func driveResponseContract(operation *openapi3.Operation, status int) *openapi3.ResponseRef {
	declared := operation.Responses.Value(strconv.Itoa(status))
	if declared == nil && status >= http.StatusOK && status < http.StatusMultipleChoices {
		declared = operation.Responses.Value("2XX")
	}

	if declared == nil {
		declared = operation.Responses.Value("default")
	}

	return declared
}

func TestDriveContentContractsAllowSuccessfulStatuses(t *testing.T) {
	t.Parallel()

	operation := loadDriveDocument(t, driveContentSchemaPath).Paths.Value(driveContentPath).Get

	for _, status := range []int{http.StatusNoContent, http.StatusPartialContent} {
		response := replay.Response{BodyRepresentation: "", Status: status, Headers: nil,
			Body: replay.Entity{Encoding: driveContractBinaryEncoding,
				Value: json.RawMessage(`""`), Matchers: nil, ContentTypePattern: "", Parts: nil}}
		validateDriveResponse(t, operation, &response)
	}
}

func driveHeaderMedia(t *testing.T, headers []replay.Pair) string {
	t.Helper()

	for _, header := range headers {
		if strings.EqualFold(header[0], "Content-Type") {
			value, _, err := mime.ParseMediaType(header[1])
			if err != nil {
				t.Fatal(err)
			}

			return value
		}
	}

	t.Fatal("Drive entity has no media type")

	return ""
}

func driveJSONValue(entity replay.Entity) (any, error) {
	body := []byte(entity.Value)

	if entity.Encoding == driveContractBinaryEncoding {
		var encoded string

		err := json.Unmarshal(entity.Value, &encoded)
		if err != nil {
			return nil, fmt.Errorf("decode Drive entity: %w", err)
		}

		body, err = base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode Drive bytes: %w", err)
		}
	}

	var complete json.RawMessage

	err := json.Unmarshal(body, &complete)
	if err != nil {
		return nil, fmt.Errorf("decode Drive JSON document: %w", err)
	}

	var value any

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()

	err = decoder.Decode(&value)
	if err != nil {
		return nil, fmt.Errorf("decode Drive JSON: %w", err)
	}

	return value, nil
}

func TestDriveGenerationHasNoDrift(t *testing.T) {
	t.Parallel()

	for _, artifact := range []generationArtifact{
		{Schema: driveModelsPath, Config: "../../pkg/dependencymodels/drive/config.yaml",
			Output: "../../pkg/dependencymodels/drive/models.gen.go"},
		{Schema: driveSchemaPath, Config: "../../pkg/dependencies/webtransport/driveapi/config.yaml",
			Output: "../../pkg/dependencies/webtransport/driveapi/client.gen.go"},
		{Schema: driveContentSchemaPath, Config: "../../pkg/dependencies/webtransport/drivecontentapi/config.yaml",
			Output: "../../pkg/dependencies/webtransport/drivecontentapi/client.gen.go"},
	} {
		t.Run(artifact.Config, func(t *testing.T) {
			t.Parallel()
			verifyGeneration(t, artifact)
		})
	}
}

func TestDriveContractsRejectPayloadDrift(t *testing.T) {
	t.Parallel()

	document := loadDriveDocument(t, driveModelsPath)

	for name, invalid := range map[string]string{
		"DriveNodeQueries":  `[{"drivewsid":"synthetic-node"}]`,
		"DriveNodeQuery":    `{"drivewsid":"synthetic-node","partialData":true}`,
		"DriveNode":         `{"size":-1}`,
		"DriveAppLibraries": `{"items":false}`,
		"DriveFolderCreation": `{"clientId":"FOLDER::UNKNOWN_ZONE::TempId-00000000-0000-0000-0000-000000000000",` +
			`"name":"synthetic"}`,
		"DriveItemChange":        `{"drivewsid":"synthetic-node"}`,
		"DriveMoveItems":         `{"items":[]}`,
		"DriveUploadRequest":     `{"filename":"synthetic","type":"FOLDER","content_type":"","size":0}`,
		"DriveUploadDestination": `{"document_id":"synthetic-document"}`,
		"DriveUploadedFile": `{"fileChecksum":"synthetic","wrappingKey":"synthetic",` +
			`"referenceChecksum":"synthetic",` + `"size":-1}`,
		"DriveDocumentData": `{"signature":"synthetic","wrapping_key":"synthetic",` +
			`"reference_signature":"synthetic",` + `"size":"0"}`,
		"DriveDocumentPath": `{"starting_document_id":"synthetic"}`,
		"DriveFileFlags":    `{"is_writable":true,"is_executable":true,"is_hidden":false}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var value any

			err := json.Unmarshal([]byte(invalid), &value)
			if err != nil {
				t.Fatal(err)
			}

			if document.Components.Schemas[name].Value.VisitJSON(value) == nil {
				t.Fatal("invalid Drive payload accepted")
			}
		})
	}
}

func TestDriveContentBindingRejectsUnissuedTargets(t *testing.T) {
	t.Parallel()

	documents := loadDriveDocuments(t)

	for _, mutation := range []string{
		"origin", mutationPath, mutationMethod, "issuer", "issuer-prefix", "issuer-method", "first",
	} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()

			exchanges := accountExchanges(t, "../replay/fixtures/synthetic/http/drive-download-data_token-binary.json")
			index := 1

			switch mutation {
			case "origin":
				exchanges[index].Request.Origin = "https://unissued.example.invalid"
			case mutationPath:
				exchanges[index].Request.Path += "/unissued"
			case mutationMethod:
				exchanges[index].Request.Method = http.MethodPost
			case "issuer":
				exchanges[0].Request.Path = "/unregistered"
			case "issuer-prefix":
				exchanges[0].Request.Path = "/unregistered" + exchanges[0].Request.Path
			case "issuer-method":
				exchanges[0].Request.Method = http.MethodPost
			case "first":
				exchanges = exchanges[1:]
				index = 0
			}

			_, err := bindDriveOperation(documents, exchanges, index)
			if !errors.Is(err, errDriveBinding) {
				t.Fatal("unissued content transfer bound to the wildcard contract")
			}
		})
	}
}

func TestDriveRouteBindingRejectsUnknownMethodOrPath(t *testing.T) {
	t.Parallel()

	documents := loadDriveDocuments(t)

	for _, mutation := range []string{mutationMethod, mutationPath} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()

			exchanges := accountExchanges(t, "../replay/fixtures/synthetic/http/drive-folder-one.json")
			if mutation == mutationMethod {
				exchanges[0].Request.Method = http.MethodGet
			} else {
				exchanges[0].Request.Path = "/unregistered"
			}

			_, err := bindDriveOperation(documents, exchanges, 0)
			if !errors.Is(err, errDriveBinding) {
				t.Fatal("unregistered Drive operation accepted")
			}
		})
	}
}

func driveIssuerStatus(status int, method string) bool {
	if method == http.MethodPost {
		return status >= http.StatusOK && status < http.StatusMultipleChoices
	}

	return status == http.StatusOK
}
