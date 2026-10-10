package contracts_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/tests/replay"
)

const (
	findMySchemaPath    = "../../api/external/findmy.openapi.yaml"
	findMyModelsPath    = "../../api/external/findmy-models.openapi.yaml"
	contractClientIDKey = "clientId"
)

var errFindMyBinding = errors.New("find my exchange has no verified contract binding")

func TestPortableFindMyExchangesMatchContracts(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, findMySchemaPath)
	authDocument := loadDriveDocument(t, "../../api/external/auth.openapi.yaml")

	paths, err := filepath.Glob("../replay/fixtures/synthetic/http/findmy-*.json")
	if err != nil {
		t.Fatal(err)
	}

	recoveryPaths, err := filepath.Glob("../replay/fixtures/synthetic/http/session-findmy-autorefresh-*.json")
	if err != nil {
		t.Fatal(err)
	}

	paths = append(paths, recoveryPaths...)

	pairs := 0
	operations := make(map[string]int)

	for _, path := range paths {
		for index, exchange := range accountExchanges(t, path) {
			selected := findMyRecoveryDocument(t, document, authDocument, exchange.Request)

			operation, err := bindFindMyOperation(selected, exchange.Request)
			if err != nil {
				t.Fatalf("%s exchange %d: %v", path, index, err)
			}

			validateFindMyExchange(t, operation, exchange)

			operations[operation.OperationID]++
			pairs++
		}
	}

	if len(paths) != 76 || pairs != 154 || len(operations) != 8 {
		t.Fatalf("Find My inventory changed: scenarios=%d pairs=%d operations=%d", len(paths), pairs, len(operations))
	}
}

func findMyRecoveryDocument(t *testing.T, document, authDocument *openapi3.T, request replay.Request) *openapi3.T {
	t.Helper()

	if request.Path != protocol.AuthLoginAuthTokenPath {
		return document
	}

	if request.Origin != protocol.AuthAccountServer0 || len(request.Query) != 0 {
		t.Fatal("Find My recovery changed the authenticated setup origin or query")
	}

	return authDocument
}

func TestFindMyCommandsPreserveOpaqueReplyContracts(t *testing.T) {
	t.Parallel()

	document := loadDriveDocument(t, findMySchemaPath)
	for _, path := range []string{
		"/fmipservice/client/web/playSound", "/fmipservice/client/web/sendMessage",
		"/fmipservice/client/web/lostDevice", "/fmipservice/client/web/remoteWipeWithUserAuth",
	} {
		operation := document.Paths.Value(path).Post
		for _, status := range []int{http.StatusOK, http.StatusCreated, http.StatusAccepted} {
			response := driveResponseContract(operation, status)
			for _, contentType := range []string{"application/json; charset=utf-8", "text/json", "application/octet-stream"} {
				media, err := contractResponseMedia(response.Value.Content,
					[]replay.Pair{{"Content-Type", contentType}})
				if err != nil {
					t.Fatal(err)
				}

				if media.Schema.Value.Format != binaryComponent {
					t.Fatal("opaque command reply lost its byte contract")
				}
			}
		}
	}

	parsed := document.Components.Schemas["FindMyAcknowledgement"].Value

	err := parsed.VisitJSON(nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, input := range []string{`{}`, `[null,false,9007199254740993]`, `"accepted"`, `9007199254740993`} {
		var value any

		err := json.Unmarshal([]byte(input), &value)
		if err != nil {
			t.Fatal(err)
		}

		err = parsed.VisitJSON(value)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func bindFindMyOperation(document *openapi3.T, request replay.Request) (*openapi3.Operation, error) {
	item := document.Paths.Value(request.Path)
	if item != nil {
		if operation := item.GetOperation(request.Method); operation != nil {
			return operation, nil
		}
	}

	return nil, errFindMyBinding
}

func validateFindMyExchange(t *testing.T, operation *openapi3.Operation, exchange replay.Exchange) {
	t.Helper()
	validateFindMyParameters(t, operation, exchange.Request)
	validateDriveRequest(t, operation, exchange.Request)

	response := driveResponseContract(operation, exchange.Response.Status)
	if response != nil && len(response.Value.Content) == 0 {
		if len(findMyEntityBytes(t, exchange.Response.Body)) != 0 {
			t.Fatal("no-content command response has an entity")
		}

		return
	}

	validateDriveResponse(t, operation, exchange.Response)
}

func validateFindMyParameters(t *testing.T, operation *openapi3.Operation, request replay.Request) {
	t.Helper()

	values := make(map[string][]string)
	for _, pair := range request.Query {
		values[pair[0]] = append(values[pair[0]], pair[1])
	}

	err := validateFindMyQuery(operation, values)
	if err != nil {
		t.Fatal(err)
	}
}

func validateFindMyQuery(operation *openapi3.Operation, query url.Values) error {
	declared := make(map[string]*openapi3.Parameter)

	for _, reference := range operation.Parameters {
		parameter := reference.Value
		if parameter.In == "query" {
			declared[parameter.Name] = parameter

			if parameter.Required && len(query[parameter.Name]) == 0 {
				return fmt.Errorf("%w: required query %s", errFindMyBinding, parameter.Name)
			}
		}
	}

	for name, values := range query {
		parameter := declared[name]
		if parameter == nil || len(values) != 1 {
			return fmt.Errorf("%w: unknown or repeated query %s", errFindMyBinding, name)
		}

		err := parameter.Schema.Value.VisitJSON(values[0])
		if err != nil {
			return fmt.Errorf("Find My query %s: %w", name, err)
		}
	}

	return nil
}

func TestFindMyGenerationHasNoDrift(t *testing.T) {
	t.Parallel()

	for _, artifact := range []generationArtifact{
		{Schema: findMyModelsPath, Config: "../../pkg/dependencymodels/findmy/config.yaml",
			Output: "../../pkg/dependencymodels/findmy/models.gen.go"},
		{Schema: findMySchemaPath, Config: "../../pkg/dependencies/webtransport/findmyapi/config.yaml",
			Output: "../../pkg/dependencies/webtransport/findmyapi/client.gen.go"},
	} {
		t.Run(filepath.Base(artifact.Output), func(t *testing.T) {
			t.Parallel()
			verifyGeneration(t, artifact)
		})
	}
}

func TestFindMyRouteAndQueryBindingRejectsUnknowns(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, findMySchemaPath)

	exchange := accountExchanges(t, "../replay/fixtures/synthetic/http/findmy-devices-one.json")[0]

	for _, mutation := range []string{"path", "method", "required", repeatedParameterControl, "unknown", "token-query"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()

			request := exchange.Request
			query := url.Values{contractClientIDKey: {"synthetic-client"}, "dsid": {"synthetic-account"}}
			mutateFindMyBinding(&request, query, mutation)

			operation, err := bindFindMyOperation(document, request)
			if err == nil {
				err = validateFindMyQuery(operation, query)
			}

			if err == nil {
				t.Fatal("invalid Find My binding accepted")
			}
		})
	}
}

func mutateFindMyBinding(request *replay.Request, query url.Values, mutation string) {
	switch mutation {
	case "path":
		request.Path += unknownContractPath
	case "method":
		request.Method = "GET"
	case "required":
		delete(query, "dsid")
	case repeatedParameterControl:
		query.Add("dsid", "another-account")
	case "unknown":
		query.Set("unexpected", "value")
	case "token-query":
		request.Path = "/setup/ws/1/fmipWebAuthenticate"
	}
}

func TestFindMyModelExamplesAndProtocolConstraints(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, findMyModelsPath)
	examples := 0

	for name, schema := range document.Components.Schemas {
		if schema.Value.Example != nil {
			err := schema.Value.VisitJSON(schema.Value.Example)
			if err != nil {
				t.Fatalf("%s example: %v", name, err)
			}

			examples++
		}
	}

	if examples != 4 {
		t.Fatalf("Find My example inventory changed: %d", examples)
	}

	for name, input := range map[string]string{
		"FindMyDevice":         `{}`,
		"FindMySoundRequest":   `{"device":"synthetic","subject":"alert","clientContext":{"fmly":false}}`,
		"FindMyMessageRequest": `{"device":"synthetic","subject":"alert","text":"note"}`,
		"FindMyLocation":       `{"latitude":91}`,
		"FindMyEraseRequest":   `{"device":"synthetic","text":"note","passcode":""}`,
		"FindMyRefreshRequest": `{"clientContext":{}}`,
	} {
		var value any

		err := json.Unmarshal([]byte(input), &value)
		if err != nil {
			t.Fatal(err)
		}

		if document.Components.Schemas[name].Value.VisitJSON(value) == nil {
			t.Fatalf("invalid %s accepted", name)
		}
	}

	context, ok := document.Components.Schemas["FindMyInitializeRequest"].Value.Example.(map[string]any)
	if !ok {
		t.Fatal("initial setup example lost its object shape")
	}

	context["serverContext"] = map[string]any{"future": true}
	if document.Components.Schemas["FindMyInitializeRequest"].Value.VisitJSON(context) == nil {
		t.Fatal("initial setup accepted a refresh context")
	}
}
