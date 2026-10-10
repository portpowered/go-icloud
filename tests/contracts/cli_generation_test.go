package contracts_test

import "testing"

func TestCLIReferenceConfigurationGeneration(t *testing.T) {
	t.Parallel()
	loadDriveDocument(t, "../../cmd/go-icloud/api/reference-login.openapi.yaml")
	verifyGeneration(t, generationArtifact{Schema: "../../cmd/go-icloud/api/reference-login.openapi.yaml",
		Config: "../../cmd/go-icloud/internal/referenceconfig/config.yaml",
		Output: "../../cmd/go-icloud/internal/referenceconfig/models.gen.go"})
}

func TestCLICommandProjectionGeneration(t *testing.T) {
	t.Parallel()
	loadDriveDocument(t, "../../cmd/go-icloud/api/command-models.openapi.yaml")
	verifyGeneration(t, generationArtifact{Schema: "../../cmd/go-icloud/api/command-models.openapi.yaml",
		Config: "../../cmd/go-icloud/internal/commandmodels/config.yaml",
		Output: "../../cmd/go-icloud/internal/commandmodels/models.gen.go"})
}

func TestCLIPhotosWatchGeneration(t *testing.T) {
	t.Parallel()
	loadDriveDocument(t, "../../cmd/go-icloud/api/photo-sync-command.openapi.yaml")
	verifyGeneration(t, generationArtifact{Schema: "../../cmd/go-icloud/api/photo-sync-command.openapi.yaml",
		Config: "../../cmd/go-icloud/internal/photosynccommand/config.yaml",
		Output: "../../cmd/go-icloud/internal/photosynccommand/models.gen.go"})
}
