package contracts_test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// SCHEMA-16: run with the same repository-relative template inputs as make
// generate-api, while keeping configuration and output in test-owned storage.
func generationCommand(t *testing.T, config, schema string) *exec.Cmd {
	t.Helper()

	absoluteSchema, err := filepath.Abs(schema)
	if err != nil {
		t.Fatal(err)
	}

	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	//nolint:gosec // SCHEMA-16: fixed generator and checked-in inputs; test-owned output paths vary.
	command := exec.CommandContext(t.Context(), "go", "run", generatorTool, "-config", config, absoluteSchema)
	command.Dir = repository

	return command
}
