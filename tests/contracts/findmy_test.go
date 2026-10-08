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

	paths, err := filepath.Glob("../replay/fixtures/synthetic/http/findmy-*.json")
	if err != nil {
		t.Fatal(err)
	}

	pairs := 0
	operations := make(map[string]int)

	for _, path := range paths {
		for index, exchange := range accountExchanges(t, path) {
			operation, err := bindFindMyOperation(document, exchange.Request)
			if err != nil {
				t.Fatalf("%s exchange %d: %v", path, index, err)
			}

			validateFindMyExchange(t, operation, exchange)

			operations[operation.OperationID]++
			pairs++
		}
	}

	if len(paths) != 36 || pairs != 68 || len(operations) != 7 {
		t.Fatalf("Find My inventory changed: scenarios=%d pairs=%d operations=%d", len(paths), pairs, len(operations))
	}
}

func TestFindMySuccessfulJSONCommandsOwnTheirMediaContract(t *testing.T) {
	t.Parallel()

	operation := loadDriveDocument(t, findMySchemaPath).Paths.Value("/fmipservice/client/web/playSound").Post
	for _, status := range []int{http.StatusCreated, http.StatusAccepted} {
		response := driveResponseContract(operation, status)

		media, err := contractResponseMedia(response.Value.Content,
			[]replay.Pair{{"Content-Type", "application/json; charset=utf-8"}})
		if err != nil {
			t.Fatal(err)
		}

		if media.Schema.Value.Format == binaryComponent || !media.Schema.Value.Type.Is("object") {
			t.Fatal("JSON acknowledgement bypassed its explicit schema")
		}

		if media.Schema.Value.VisitJSON([]any{"invalid acknowledgement"}) == nil {
			t.Fatal("non-object JSON acknowledgement accepted")
		}

		err = media.Schema.Value.VisitJSON(map[string]any{"future": true})
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
		{Schema: findMySchemaPath, Config: "../../internal/findmyapi/config.yaml",
			Output: "../../internal/findmyapi/client.gen.go"},
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

	for _, mutation := range []string{"path", "method", "required", "repeated", "unknown", "token-query"} {
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
		request.Path += "/unknown"
	case "method":
		request.Method = "GET"
	case "required":
		delete(query, "dsid")
	case "repeated":
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
