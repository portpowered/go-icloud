package contracts_test

import (
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestSavedSessionWireContracts(t *testing.T) {
	t.Parallel()

	document := loadDriveDocument(t, "../../api/external/auth.openapi.yaml")

	for _, name := range []string{"auth-authenticate-cloudkit-discovery",
		"auth-authenticate-cached", "auth-authenticate-paused", "auth-authenticate-refresh",
		"auth-authenticate-untrusted-refresh", "auth-authenticate-stale-token", "auth-token-cookie-rotation",
		"auth-authenticate-validation-201", "auth-authenticate-refresh-202",
		"auth-authenticate-empty-headers", "auth-authenticate-empty-headers-refresh",
		"auth-authenticate-quoted-cookie", "auth-authenticate-quoted-cookie-rotation", "auth-authenticate-explicit-cookie",
		"auth-terms-refused", "auth-token-login-needs-2fa", "auth-authenticate-untrusted-no-password"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			for _, exchange := range accountExchanges(t, filepath.Join("../replay/fixtures/synthetic/http", name+".json")) {
				item := document.Paths.Value(exchange.Request.Path)
				if item == nil {
					t.Fatal("unbound authentication request")
				}

				operation := item.GetOperation(exchange.Request.Method)
				if operation == nil || exchange.Request.Origin != protocol.AuthAccountServer0 || len(exchange.Request.Query) != 0 {
					t.Fatal("authentication method, origin or query changed")
				}

				media := operation.RequestBody.Value.Content[protocol.AuthMediaApplicationJson]

				err := media.Schema.Value.VisitJSON(accountJSONBody(t, exchange.Request.Body))
				if err != nil {
					t.Fatal(err)
				}

				validateSavedAuthResponse(t, operation, exchange.Response)
			}
		})
	}
}

func TestSavedSessionGenerationHasNoDrift(t *testing.T) {
	t.Parallel()

	for _, artifact := range []generationArtifact{
		{Schema: "../../api/external/auth-models.openapi.yaml", Config: "../../pkg/dependencymodels/auth/config.yaml",
			Output: "../../pkg/dependencymodels/auth/models.gen.go"},
		{Schema: "../../api/external/auth.openapi.yaml", Config: "../../internal/authapi/config.yaml",
			Output: "../../internal/authapi/client.gen.go"},
	} {
		t.Run(artifact.Output, func(t *testing.T) { t.Parallel(); verifyGeneration(t, artifact) })
	}
}

func TestAuthTokenRequiredFields(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, "../../api/external/auth-models.openapi.yaml")
	schema := document.Components.Schemas["AuthTokenLoginRequest"].Value

	for _, field := range []string{"accountCountryCode", "dsWebAuthToken", "extended_login", "trustToken"} {
		t.Run(field, func(t *testing.T) {
			t.Parallel()

			value := map[string]any{"accountCountryCode": nil, "dsWebAuthToken": "synthetic-saved-token",
				"extended_login": true, "trustToken": ""}
			delete(value, field)

			err := schema.VisitJSON(value)
			if err == nil {
				t.Fatal("missing token-login field accepted")
			}
		})
	}
}

func validateSavedAuthResponse(t *testing.T, operation *openapi3.Operation, response *replay.Response) {
	t.Helper()

	if response.Status < 200 || response.Status >= 300 {
		validateDriveResponse(t, operation, response)

		return
	}

	mediaType := accountMediaType(t, response)
	schema := operation.Responses.Value("2XX").Value.Content[mediaType].Schema.Value

	err := schema.VisitJSON(accountJSONBody(t, response.Body))
	if err != nil {
		t.Fatal(err)
	}
}
