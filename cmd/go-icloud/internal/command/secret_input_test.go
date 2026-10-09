//nolint:testpackage // Verifies the private secret-input cancellation and bounds contract.
package command

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestSecretSources(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		text        string
		stdin       bool
		environment Environment
		want        string
		failure     bool
	}{
		{name: "environment", environment: func(string) string { return " synthetic password " }, want: " synthetic password "},
		{name: "line", text: " synthetic password \r\nignored", stdin: true, want: " synthetic password "},
		{name: "end of file", text: "synthetic-code", stdin: true, want: "synthetic-code"},
		{name: "empty", stdin: true, failure: true},
		{name: "missing environment", failure: true},
		{name: "bounded input", text: strings.Repeat("x", maximumSecretBytes+1), stdin: true, failure: true},
		{name: "bounded environment", environment: func(string) string { return strings.Repeat("x", maximumSecretBytes+1) }, failure: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value, err := readSecret(t.Context(), io.NopCloser(strings.NewReader(test.text)), test.environment, "SECRET", test.stdin)
			if test.failure {
				if !errors.Is(err, errSecret) || value != "" {
					t.Fatal("secret input did not reject unavailable or oversized data")
				}
			} else if err != nil || value != test.want {
				t.Fatal("secret input changed the selected value")
			}
		})
	}
}

func TestSecretCancellationClosesBlockedInput(t *testing.T) {
	t.Parallel()
	reader, writer := io.Pipe()
	t.Cleanup(func() { _ = writer.Close() })
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	started := make(chan struct{})
	go func() {
		_, err := readSecret(ctx, &startedReader{ReadCloser: reader, started: started}, nil, "SECRET", true)
		result <- err
	}()
	<-started
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatal("secret read lost cancellation")
	}
}

type startedReader struct {
	io.ReadCloser
	started chan struct{}
}

func (reader *startedReader) Read(data []byte) (int, error) {
	close(reader.started)
	return reader.ReadCloser.Read(data)
}

func TestSecretPreCancellationAvoidsReading(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := readSecret(ctx, nil, func(string) string { t.Fatal("environment read after cancellation"); return "" }, "SECRET", false)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("secret read lost prior cancellation")
	}
}
