//nolint:testpackage // Unit tests verify private credential export without constructing an SDK transport.
package command

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCredentialExportExplicitAndPrivate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := filepath.Join(root, "source.json")
	destination := filepath.Join(root, "export.json")
	data := []byte(`{"clientID":"synthetic-client","sessionToken":"synthetic-secret"}`)

	err := os.WriteFile(source, data, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	var output, diagnostic bytes.Buffer

	err = Run(t.Context(), nil, []string{"--session", source, "--export-session", destination,
		"credentials-export"}, &output, &diagnostic)
	if err != nil {
		t.Fatal(err)
	}

	actual, err := os.ReadFile(filepath.Clean(destination))
	if err != nil || !bytes.Equal(actual, data) {
		t.Fatalf("export did not preserve caller credentials: %v", err)
	}

	if strings.Contains(output.String(), "synthetic-secret") || diagnostic.Len() != 0 {
		t.Fatal("export disclosed credentials in ordinary output")
	}

	assertNoTemporarySessions(t, root)
}

func TestCredentialExportRequiresDestination(t *testing.T) {
	t.Parallel()

	var output, diagnostic bytes.Buffer

	err := Run(t.Context(), nil, []string{"--session", "unused.json", "credentials-export"}, &output, &diagnostic)
	if !errors.Is(err, errExportDestination) || output.Len() != 0 {
		t.Fatalf("export did not require explicit destination: %v", err)
	}
}

func TestCancelledCredentialExportPreservesDestination(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := filepath.Join(root, "source.json")
	destination := filepath.Join(root, "export.json")

	data := []byte(`{"clientID":"synthetic-client","sessionToken":"synthetic-secret"}`)

	for path, content := range map[string][]byte{source: data, destination: []byte("previous credentials")} {
		err := os.WriteFile(path, content, 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var config options

	config.session, config.exportSession = source, destination

	err := exportCredentials(ctx, config, &bytes.Buffer{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("export lost cancellation: %v", err)
	}

	actual, err := os.ReadFile(filepath.Clean(destination))
	if err != nil || string(actual) != "previous credentials" {
		t.Fatal("canceled export changed existing credentials")
	}
}
