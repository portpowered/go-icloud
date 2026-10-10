package contracts_test

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// SCHEMA-03: provider JSON remains unrestricted through its named schema owner.
func TestBridgeExchangeOpaqueSchemaAcceptsEveryJSONKind(t *testing.T) {
	t.Parallel()

	loader := openapi3.NewLoader()

	document, err := loader.LoadFromFile("../../api/external/bridge-models.openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	err = document.Validate(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	owner := document.Components.Schemas["BridgeExchangeAkdata"]

	field := document.Components.Schemas["BridgeExchange"].Value.Properties["akdata"]

	if field.Ref != "#/components/schemas/BridgeExchangeAkdata" || field.Value != owner.Value {
		t.Fatal("bridge exchange field lost its exact opaque schema owner")
	}

	for _, value := range []any{nil, true, 1.5, "source", []any{nil, true}, map[string]any{"future": nil}} {
		err = field.Value.VisitJSON(value)
		if err != nil {
			t.Fatalf("provider JSON kind %T was restricted: %v", value, err)
		}
	}
}
