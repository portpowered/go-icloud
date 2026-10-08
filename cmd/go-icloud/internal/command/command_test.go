//nolint:testpackage // Unit tests exercise private output sanitization and session error boundaries.
package command

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func TestHelpWithoutCredentials(t *testing.T) {
	t.Parallel()

	var output, diagnostic bytes.Buffer

	err := Run(t.Context(), nil, []string{"--help"}, &output, &diagnostic)
	if err != nil {
		t.Fatal(err)
	}

	if output.Len() != 0 || !strings.Contains(diagnostic.String(), "drive-libraries") {
		t.Fatal("help changed")
	}
}

func TestMissingSessionPreservesCause(t *testing.T) {
	t.Parallel()
	_, err := loadSession(filepath.Join(t.TempDir(), "missing.json"))

	var failure *SessionError

	if !errors.As(err, &failure) || !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("missing session lost its cause")
	}

	if strings.Contains(err.Error(), "missing.json") {
		t.Fatal("session error disclosed the private path")
	}
}

func TestResponseHeadersOmitted(t *testing.T) {
	t.Parallel()

	var result icloud.GetAccountDevicesResult

	result.Metadata.Headers = []icloud.Header{{Name: "Set-Cookie", Value: "synthetic-secret"}}

	var output bytes.Buffer

	err := writeResult(&output, &result)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(output.String(), "synthetic-secret") || strings.Contains(output.String(), "metadata") {
		t.Fatal("CLI disclosed response authentication metadata")
	}
}

func TestOutputFailurePreservesCause(t *testing.T) {
	t.Parallel()

	err := writeResult(failedWriter{}, []string{"synthetic"})
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("output error lost its cause")
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestCancelledRead(t *testing.T) {
	t.Parallel()

	client, err := icloud.New()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var auth icloud.AuthContext

	auth.AccountServiceURL = "https://account.example.invalid"
	auth.AccountID = "synthetic-account"
	auth.ClientID = "synthetic-client"

	var config options

	config.operation = "account-devices"

	_, err = read(ctx, client, auth, config)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled CLI read: %v", err)
	}
}
