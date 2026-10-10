package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModuleReplacementRejected(t *testing.T) {
	t.Parallel()
	root := testRepository(t, "module example.invalid/consumer\n\ngo 1.24.0\n"+
		"\nreplace example.invalid/dependency => ../dependency\n", "package consumer\n")

	err := audit(t.Context(), root)
	if !errors.Is(err, errReplacement) {
		t.Fatalf("replace directive escaped module check: %v", err)
	}
}

func TestNestedModuleReplacementRejected(t *testing.T) {
	t.Parallel()
	root := testRepository(t, "module example.invalid/consumer\n\ngo 1.24.0\n", "package consumer\n")
	child := filepath.Join(root, "cmd", "consumer")

	err := os.MkdirAll(child, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, child, "go.mod", "module example.invalid/cli\n\ngo 1.24.0\n"+
		"\nreplace example.invalid/dependency => ../dependency\n")
	runTestCommand(t, root, "git", "add", ".")

	err = audit(t.Context(), root)
	if !errors.Is(err, errReplacement) {
		t.Fatalf("nested CLI replacement escaped module check: %v", err)
	}
}

func TestFormattingRejected(t *testing.T) {
	t.Parallel()
	root := testRepository(t, "module example.invalid/consumer\n\ngo 1.24.0\n", "package consumer\n\nvar Value=1\n")

	err := audit(t.Context(), root)
	if !errors.Is(err, errFormatting) {
		t.Fatalf("unformatted source escaped module check: %v", err)
	}
}

func TestFormattedIndependentModuleAccepted(t *testing.T) {
	t.Parallel()
	root := testRepository(t, "module example.invalid/consumer\n\ngo 1.24.0\n", "package consumer\n")

	err := audit(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDefaultCommandRejectsFormatting(t *testing.T) {
	t.Parallel()
	root := testRepository(t, "module example.invalid/consumer\n\ngo 1.24.0\n", "package consumer\n\nvar Value=1\n")

	source, err := filepath.Abs("main.go")
	if err != nil {
		t.Fatal(err)
	}

	_, err = run(t.Context(), root, "go", "run", source)
	if err == nil || !strings.Contains(err.Error(), errFormatting.Error()) {
		t.Fatalf("default CI command accepted formatting drift: %v", err)
	}
}

func testRepository(t *testing.T, module, source string) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", module)
	writeTestFile(t, root, "consumer.go", source)
	runTestCommand(t, root, "git", "init", "--quiet")
	runTestCommand(t, root, "git", "add", ".")

	return root
}

func writeTestFile(t *testing.T, root, name, data string) {
	t.Helper()

	err := os.WriteFile(filepath.Join(root, name), []byte(data), 0o600)
	if err != nil {
		t.Fatal(err)
	}
}

func runTestCommand(t *testing.T, root, executable string, arguments ...string) {
	t.Helper()

	_, err := run(t.Context(), root, executable, arguments...)
	if err != nil {
		t.Fatal(err)
	}
}
