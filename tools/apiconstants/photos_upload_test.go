package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestPhotosUploadProtocolConstantsHaveNoDrift(t *testing.T) {
	t.Parallel()

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	document, err := loader.LoadFromFile("../../api/external/photos-upload.openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	values, err := externalDocumentConstants(document, document, true)
	if err != nil {
		t.Fatal(err)
	}

	actual, err := renderConstants(values, "PhotosUpload")
	if err != nil {
		t.Fatal(err)
	}

	expected, err := os.ReadFile("../../internal/protocol/photosupload.gen.go")
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatal("Photos upload protocol constants drifted; run make generate-api")
	}
}
