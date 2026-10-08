package main

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func findMyConstantDocument(t *testing.T, models bool) *openapi3.T {
	t.Helper()

	path := "../../api/external/findmy.openapi.yaml"
	if models {
		path = "../../api/external/findmy-models.openapi.yaml"
	}

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

func TestFindMyProtocolConstantsHaveNoDrift(t *testing.T) {
	t.Parallel()

	values, err := externalConstants(findMyConstantDocument(t, false), findMyConstantDocument(t, true))
	if err != nil {
		t.Fatal(err)
	}

	actual, err := renderConstants(values, "FindMy")
	if err != nil {
		t.Fatal(err)
	}

	expected, err := os.ReadFile("../../internal/protocol/findmy.gen.go")
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatal("Find My protocol constants drifted; run make generate-api")
	}
}

func TestFindMyProtocolConstantsFollowModelsAndRejectCollision(t *testing.T) {
	t.Parallel()
	routes, models := findMyConstantDocument(t, false), findMyConstantDocument(t, true)
	device := models.Components.Schemas["FindMyDevice"].Value
	device.Properties["futureName"] = device.Properties["name"]
	delete(device.Properties, "name")

	values, err := externalConstants(routes, models)
	if err != nil || values["FindMyDeviceFutureName"] != "futureName" {
		t.Fatal("Find My field constants did not follow the schema")
	}

	if _, exists := values["FindMyDeviceName"]; exists {
		t.Fatal("obsolete Find My field constant survived")
	}

	collision := *models.Components.Schemas["FindMyInitializeRequest"].Value
	collision.Properties = openapi3.Schemas{"Path": collision.Properties["clientContext"]}
	reference := *models.Components.Schemas["FindMyInitializeRequest"]
	reference.Ref, reference.Value = "", &collision
	models.Components.Schemas["FindMyInitialize"] = &reference

	_, err = externalConstants(routes, models)
	if !errors.Is(err, errConstant) {
		t.Fatal("ambiguous Find My operation/property constant accepted")
	}
}
