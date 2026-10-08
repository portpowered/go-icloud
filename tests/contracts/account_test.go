package contracts_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/account"
	"github.com/portpowered/go-icloud/tests/replay"
)

const (
	accountSchemaPath = "../../api/external/account.openapi.yaml"
	generatorTool     = "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0"
	contractFileMode  = 0o600
	binaryComponent   = "binary"
)

func accountSchema(t *testing.T) *openapi3.T {
	t.Helper()

	loader := openapi3.NewLoader()

	document, err := loader.LoadFromFile(accountSchemaPath)
	if err != nil {
		t.Fatal(err)
	}

	err = document.Validate(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	return document
}

func TestAccountDocumentExamplesAndConstraints(t *testing.T) {
	t.Parallel()

	document := accountSchema(t)
	examples := accountExamples(t, document)

	if examples != 10 {
		t.Fatalf("account example count changed: %d", examples)
	}

	validateAccountConstraints(t, document)
}

func accountExamples(t *testing.T, document *openapi3.T) int {
	t.Helper()

	examples := 0

	for _, path := range document.Paths.Map() {
		for _, operation := range path.Operations() {
			for _, response := range operation.Responses.Map() {
				for _, media := range response.Value.Content {
					if media.Example != nil {
						err := media.Schema.Value.VisitJSON(media.Example)
						if err != nil {
							t.Fatal(err)
						}

						examples++
					}
				}
			}
		}
	}

	return examples
}

func validateAccountConstraints(t *testing.T, document *openapi3.T) {
	t.Helper()

	for name, invalid := range map[string]string{
		"AccountDevicesResponse": `{}`,
		"AccountDevice":          `{"name":false}`,
		"AccountStorageResponse": `{"storageUsageInfo":{"usedStorageInBytes":-1,"totalStorageInBytes":100}}`,
		"AccountStorageUsage":    `{"usedStorageInBytes":"15","totalStorageInBytes":100}`,
		"AccountQuota":           `{"overQuota":"true"}`,
		"AccountMediaUsage":      `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var value any

			err := json.Unmarshal([]byte(invalid), &value)
			if err != nil {
				t.Fatal(err)
			}

			if document.Components.Schemas[name].Value.VisitJSON(value) == nil {
				t.Fatal("invalid known wire payload accepted")
			}
		})
	}
}

func TestPortableAccountResponsesMatchSchemas(t *testing.T) {
	t.Parallel()

	document := accountSchema(t)

	paths, err := filepath.Glob("../replay/fixtures/synthetic/http/account-*.json")
	if err != nil {
		t.Fatal(err)
	}

	pairs := 0

	for _, path := range paths {
		exchanges := accountExchanges(t, path)

		for _, exchange := range exchanges {
			validateAccountExchange(t, document, exchange)

			pairs++
		}
	}

	if len(paths) != 18 || pairs != 24 {
		t.Fatalf("account fixture counts changed: scenarios=%d pairs=%d", len(paths), pairs)
	}
}

func accountExchanges(t *testing.T, path string) []replay.Exchange {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	var fixture struct {
		Exchanges []replay.Exchange `json:"exchanges"`
	}

	err = json.Unmarshal(data, &fixture)
	if err != nil {
		t.Fatal(err)
	}

	return fixture.Exchanges
}

func validateAccountExchange(t *testing.T, document *openapi3.T, exchange replay.Exchange) {
	t.Helper()

	path := exchange.Request.Path

	planPath := regexp.MustCompile(`^/acsegateway/v3/accounts/[^/]+/subscriptions/features/cloud\.storage/plan-summary$`)
	if planPath.MatchString(path) {
		path = "/acsegateway/v3/accounts/{dsid}/subscriptions/features/cloud.storage/plan-summary"
	}

	item := document.Paths.Value(path)
	if item == nil || item.GetOperation(exchange.Request.Method) == nil {
		t.Fatalf("unregistered account exchange: %s %s", exchange.Request.Method, path)
	}

	component := accountComponent(path)
	if component == "" {
		t.Fatalf("account path has no explicit response binding: %s", path)
	}

	if exchange.Response.Status != 200 {
		component = "AccountError"
	}

	schema := accountResponseSchema(t, item.GetOperation(exchange.Request.Method), exchange.Response, component)

	if component == binaryComponent {
		return // Member-photo bodies are uninterpreted binary, including empty content.
	}

	value := accountJSONBody(t, exchange.Response.Body)

	err := schema.VisitJSON(value)
	if err != nil {
		t.Fatalf("%s: %v", component, err)
	}
}

func accountResponseSchema(t *testing.T, operation *openapi3.Operation,
	response *replay.Response, component string,
) *openapi3.Schema {
	t.Helper()

	declared := operation.Responses.Value(strconv.Itoa(response.Status))
	if declared == nil {
		declared = operation.Responses.Value("default")
	}

	if declared == nil || declared.Value == nil {
		t.Fatal("account status has no response schema")
	}

	media := declared.Value.Content[accountMediaType(t, response)]
	if media == nil {
		media = declared.Value.Content["*/*"]
	}

	if media == nil || media.Schema == nil || media.Schema.Value == nil {
		t.Fatal("account media has no response schema")
	}

	validateAccountResponseOwner(t, media.Schema, component)

	return media.Schema.Value
}

func validateAccountResponseOwner(t *testing.T, schema *openapi3.SchemaRef, component string) {
	t.Helper()

	if component == binaryComponent {
		if schema.Value.Format != binaryComponent || !schema.Value.Type.Is("string") {
			t.Fatal("member-photo operation lost its binary response contract")
		}
	} else if schema.Ref != "#/components/schemas/"+component {
		t.Fatalf("account response owner changed: %s", schema.Ref)
	}
}

func accountMediaType(t *testing.T, response *replay.Response) string {
	t.Helper()

	for _, header := range response.Headers {
		if strings.EqualFold(header[0], "content-type") {
			value, _, err := mime.ParseMediaType(header[1])
			if err != nil {
				t.Fatal(err)
			}

			return value
		}
	}

	t.Fatal("account fixture has no response media type")

	return ""
}

func accountJSONBody(t *testing.T, entity replay.Entity) any {
	t.Helper()

	if entity.Encoding != "base64" {
		t.Fatalf("unsupported account response encoding: %s", entity.Encoding)
	}

	var encoded string

	err := json.Unmarshal(entity.Value, &encoded)
	if err != nil {
		t.Fatal(err)
	}

	body, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}

	var value any

	err = json.Unmarshal(body, &value)
	if err != nil {
		t.Fatal(err)
	}

	return value
}

func accountComponent(path string) string {
	switch path {
	case "/setup/web/device/getDevices":
		return "AccountDevicesResponse"
	case "/setup/web/family/getFamilyDetails":
		return "AccountFamilyResponse"
	case "/setup/ws/1/storageUsageInfo":
		return "AccountStorageResponse"
	case "/acsegateway/v3/accounts/{dsid}/subscriptions/features/cloud.storage/plan-summary":
		return "AccountPlanSummary"
	case "/setup/web/family/getMemberPhoto":
		return binaryComponent
	default:
		return ""
	}
}

func TestGeneratedUnknownMetadataPreservesNumbersAndNull(t *testing.T) {
	t.Parallel()

	var value account.AccountDevicesResponse

	err := json.Unmarshal([]byte(`{"devices":[{"name":"synthetic","future":{"big":9007199254740993,"nil":null}}],`+
		`"futureAccount":[true,null,9007199254740993]}`), &value)
	if err != nil {
		t.Fatal(err)
	}

	if string(value.Devices[0].AdditionalProperties["future"]) != `{"big":9007199254740993,"nil":null}` ||
		string(value.AdditionalProperties["futureAccount"]) != `[true,null,9007199254740993]` {
		t.Fatal("unknown metadata lost type, precision or null")
	}
}

func TestGeneratedNamedUnknownValuesPreservePresence(t *testing.T) {
	t.Parallel()

	for name, target := range map[string]any{
		"family": new(account.AccountFamilyMember),
		"error":  new(account.AccountError),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			input := `{"familyId":null,"ageClassification":null,"future":null}`
			if name == "error" {
				input = `{"error":null,"errorCode":null,"future":null}`
			}

			err := json.Unmarshal([]byte(input), target)
			if err != nil {
				t.Fatal(err)
			}

			encoded, err := json.Marshal(target)
			if err != nil {
				t.Fatal(err)
			}

			var actual map[string]json.RawMessage

			err = json.Unmarshal(encoded, &actual)
			if err != nil || len(actual) != 3 {
				t.Fatalf("named JSON null or omitted fields changed: %s", encoded)
			}

			for _, raw := range actual {
				if string(raw) != "null" {
					t.Fatal("explicit null lost")
				}
			}
		})
	}
}

func TestAccountGenerationHasNoDrift(t *testing.T) {
	t.Parallel()

	output := filepath.Join(t.TempDir(), "models.go")
	config := filepath.Join(t.TempDir(), "config.json")
	settings := map[string]any{
		"package": "account", "generate": map[string]bool{"models": true},
		"output": output, "output-options": map[string]bool{"skip-prune": true, "nullable-type": true},
	}

	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(config, data, contractFileMode)
	if err != nil {
		t.Fatal(err)
	}

	//nolint:gosec // SCHEMA-16: fixed generator and schema; only test-owned temporary output paths vary.
	command := exec.CommandContext(t.Context(), "go", "run", generatorTool, "-config", config, accountSchemaPath)

	log, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("schema generation: %v: %s", err, log)
	}

	actual, err := os.ReadFile(filepath.Clean(output))
	if err != nil {
		t.Fatal(err)
	}

	expected, err := os.ReadFile("../../pkg/dependencymodels/account/models.gen.go")
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatal("account generated artifacts drifted; run make generate-api")
	}
}
