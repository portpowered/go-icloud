package contracts_test

import (
	"context"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestLocalPhotoModelGeneration(t *testing.T) {
	t.Parallel()

	artifacts := []generationArtifact{
		{Schema: photoSyncModelsPath, Config: "../../pkg/photosync/config.yaml",
			Output: "../../pkg/photosync/models.gen.go"},
		{Schema: photoMaterializeModelsPath, Config: "../../internal/photomaterialize/config.yaml",
			Output: "../../internal/photomaterialize/models.gen.go"},
	}
	for _, artifact := range artifacts {
		t.Run(artifact.Schema, func(t *testing.T) { t.Parallel(); verifyGeneration(t, artifact) })
	}
}

func TestLocalPhotoSchemasValidate(t *testing.T) {
	t.Parallel()

	for _, path := range []string{photoSyncModelsPath, photoMaterializeModelsPath} {
		loader := openapi3.NewLoader()
		loader.IsExternalRefsAllowed = true

		document, err := loader.LoadFromFile(path)
		if err != nil {
			t.Fatal(err)
		}

		err = document.Validate(context.Background())

		if err != nil {
			t.Fatal(err)
		}
	}
}
