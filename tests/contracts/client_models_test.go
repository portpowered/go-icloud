package contracts_test

import (
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

	if examples != 2 {
		t.Fatalf("public model example inventory changed: %d", examples)
	}
}
