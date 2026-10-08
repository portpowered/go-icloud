package contracts_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRemindersProtobufSourceAndGeneration(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"reminders.proto", "versioned_document.proto"} {
		canonical, err := os.ReadFile(filepath.Clean(filepath.Join("../../api/external/reminders-proto", name)))
		if err != nil {
			t.Fatal(err)
		}

		source, err := os.ReadFile(filepath.Clean(filepath.Join(
			"../../.reference/pyicloud-live/pyicloud/services/reminders/protobuf", name)))
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(canonical, source) {
			t.Fatalf("pinned Reminders protocol changed: %s", name)
		}
	}

	output := t.TempDir()
	//nolint:gosec // SCHEMA-16: pinned generator and checked-in inputs; only test-owned output varies.
	command := exec.CommandContext(t.Context(), "go", "run", "github.com/bufbuild/buf/cmd/buf@v1.47.2",
		"generate", "api/external/reminders-proto", "--template", "api/external/reminders-proto/buf.gen.yaml",
		"-o", output)
	command.Dir = "../.."

	data, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("generate Reminders protobuf: %v\n%s", err, data)
	}

	for _, name := range []string{"reminders.pb.go", "versioned_document.pb.go"} {
		generated, err := os.ReadFile(filepath.Clean(filepath.Join(output, "internal/reminderstext/pb", name)))
		if err != nil {
			t.Fatal(err)
		}

		checkedIn, err := os.ReadFile(filepath.Clean(filepath.Join("../../internal/reminderstext/pb", name)))
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(generated, checkedIn) {
			t.Fatalf("Reminders protobuf generation drift: %s", name)
		}
	}
}
