//nolint:testpackage // Verifies the private secret-input cancellation and bounds contract.
package command

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

const (
	syntheticPasswordWithSpaces = " synthetic password "
	syntheticSecretCode         = "synthetic-code"
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
		{name: "environment", text: "", stdin: false,
			environment: func(string) string { return syntheticPasswordWithSpaces },
			want:        syntheticPasswordWithSpaces, failure: false},
		{name: "line", text: syntheticPasswordWithSpaces + "\r\nignored", stdin: true,
			environment: nil, want: syntheticPasswordWithSpaces, failure: false},
		{name: "end of file", text: syntheticSecretCode, stdin: true,
			environment: nil, want: syntheticSecretCode, failure: false},
		{name: "empty", text: "", stdin: true, environment: nil, want: "", failure: true},
		{name: "missing environment", text: "", stdin: false, environment: nil, want: "", failure: true},
		{name: "bounded input", text: strings.Repeat("x", maximumSecretBytes+1), stdin: true,
			environment: nil, want: "", failure: true},
		{name: "bounded environment", text: "", stdin: false,
			environment: func(string) string { return strings.Repeat("x", maximumSecretBytes+1) }, want: "", failure: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := io.NopCloser(strings.NewReader(test.text))
			value, err := readSecret(t.Context(), input, test.environment, "SECRET", test.stdin)
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
	environment := func(string) string {
		t.Fatal("environment read after cancellation")
		return ""
	}
	_, err := readSecret(ctx, nil, environment, "SECRET", false)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("secret read lost prior cancellation")
	}
}
