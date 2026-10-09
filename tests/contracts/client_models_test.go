package contracts_test

import (
	"encoding/json"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestPublicClientModelExamples(t *testing.T) {
	t.Parallel()

	loader := openapi3.NewLoader()

	document, err := loader.LoadFromFile("../../api/client-models.openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	err = document.Validate(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	examples := 0

	for name, reference := range document.Components.Schemas {
		if reference.Value.Example == nil {
			continue
		}

		err = reference.Value.VisitJSON(reference.Value.Example)
		if err != nil {
			t.Fatalf("public model example %s: %v", name, err)
		}

		examples++
	}

	if examples != 170 {
		t.Fatalf("public model example inventory changed: %d", examples)
	}
}

func TestUnknownMetadataSchemasAcceptEveryJSONKind(t *testing.T) {
	t.Parallel()

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	for _, path := range []string{"../../api/client-models.openapi.yaml", accountModelsPath} {
		document, err := loader.LoadFromFile(path)
		if err != nil {
			t.Fatal(err)
		}

		for _, body := range []string{`null`, `true`, `1`, `"text"`,
			`{"future":null}`, `[true,null,{"future":null},[null]]`} {
			var value any

			err = json.Unmarshal([]byte(body), &value)
			if err != nil {
				t.Fatal(err)
			}

			err = document.Components.Schemas["UnknownJSONValue"].Value.VisitJSON(value)
			if err != nil {
				t.Fatalf("unknown JSON schema %s rejects %s: %v", path, body, err)
			}
		}
	}
}
