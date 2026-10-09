package contracts_test

import "testing"

func TestCLIReferenceConfigurationGeneration(t *testing.T) {
	t.Parallel()
	loadDriveDocument(t, "../../cmd/go-icloud/api/reference-login.openapi.yaml")
	verifyGeneration(t, generationArtifact{Schema: "../../cmd/go-icloud/api/reference-login.openapi.yaml",
		Config: "../../cmd/go-icloud/internal/referenceconfig/config.yaml",
		Output: "../../cmd/go-icloud/internal/referenceconfig/models.gen.go"})
}
