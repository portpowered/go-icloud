package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestPhotosProtocolConstantsHaveNoDrift(t *testing.T) {
	t.Parallel()

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	document, err := loader.LoadFromFile("../../api/external/photos.openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	models, err := loader.LoadFromFile("../../api/external/cloudkit-models.openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	values, err := externalConstants(document, models)
	if err != nil {
		t.Fatal(err)
	}

	actual, err := renderConstants(values, "Photos")
	if err != nil {
		t.Fatal(err)
	}

	expected, err := os.ReadFile("../../internal/protocol/photos.gen.go")
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatal("Photos protocol constants drifted; run make generate-api")
	}
}
