package main

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestAccountProtocolConstantsHaveNoDrift(t *testing.T) {
	t.Parallel()

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	document, err := loader.LoadFromFile("../../api/external/account.openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	err = document.Validate(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	actual, err := constantsSource(document)
	if err != nil {
		t.Fatal(err)
	}

	expected, err := os.ReadFile("../../internal/protocol/account.gen.go")
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatal("protocol constants drifted; run make generate-api")
	}
}

func TestAccountProtocolConstantsRejectAmbiguousIdentifiers(t *testing.T) {
	t.Parallel()

	for _, operationID := range []string{"not-an-identifier", "same"} {
		t.Run(operationID, func(t *testing.T) {
			t.Parallel()

			loader := openapi3.NewLoader()
			loader.IsExternalRefsAllowed = true

			document, err := loader.LoadFromFile("../../api/external/account.openapi.yaml")
			if err != nil {
				t.Fatal(err)
			}

			for _, item := range document.Paths.Map() {
				for _, operation := range item.Operations() {
					operation.OperationID = operationID
				}
			}

			_, err = constantsSource(document)
			if !errors.Is(err, errConstant) {
				t.Fatal("invalid or duplicate operation constants were emitted")
			}
		})
	}
}

func TestAccountProtocolHeaderNamesComeFromSchema(t *testing.T) {
	t.Parallel()

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	document, err := loader.LoadFromFile("../../api/external/account.openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	header := document.Components.Headers["Content-Type"]
	delete(document.Components.Headers, "Content-Type")
	document.Components.Headers["X-Synthetic-Media"] = header

	values, err := accountConstants(document)
	if err != nil {
		t.Fatal(err)
	}

	if _, exists := values["HTTPContentTypeName"]; exists || values["HTTPXSyntheticMediaName"] != "X-Synthetic-Media" {
		t.Fatal("header constants did not follow their canonical schema declaration")
	}
}
