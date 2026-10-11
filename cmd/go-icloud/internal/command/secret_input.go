package command

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/term"
	"io"
	"os"
	"strings"
)

const maximumSecretBytes = 4096

var errSecret = errors.New("provide the documented secret environment variable or --secret-stdin")

// Environment reads caller-selected secret and account configuration values.
type Environment func(name string) string

type secretRead struct {
	value string
	err   error
}

func readSecret(ctx context.Context, input io.ReadCloser, environment Environment,
	name string, stdin bool,
) (string, error) {
	contextErr := ctx.Err()
	if contextErr != nil {
		return "", fmt.Errorf(secretInputCanceledError, contextErr)
	}

	if !stdin {
		if environment == nil {
			return "", errSecret
		}

		if value := environment(name); value != "" && len(value) <= maximumSecretBytes {
			return value, nil
		}

		return "", errSecret
	}

	if input == nil {
		return "", errSecret
	}

	result := make(chan secretRead, 1)
	go func() { result <- readSecretLine(input) }()

	select {
	case <-ctx.Done():
		closeErr := input.Close()

		<-result

		return "", errors.Join(fmt.Errorf(secretInputCanceledError, ctx.Err()), closeErr)
	case read := <-result:
		return read.value, read.err
	}
}

func readSecretLine(input io.Reader) secretRead {
	if file, ok := input.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		data, err := term.ReadPassword(int(file.Fd()))
		if err != nil {
			return secretRead{value: "", err: fmt.Errorf("read terminal secret: %w", err)}
		}

		if len(data) == 0 || len(data) > maximumSecretBytes {
			return secretRead{value: "", err: errSecret}
		}

		return secretRead{value: string(data), err: nil}
	}
	// Do not read ahead: interactive login consumes account, password and code
	// consecutively from the same input, including piped automation.
	var (
		data string
		err  error
	)

	for len(data) <= maximumSecretBytes {
		var next [1]byte

		_, err = io.ReadFull(input, next[:])
		if err != nil {
			break
		}

		data += string(next[:])

		if next[0] == '\n' {
			break
		}
	}

	if err != nil && !errors.Is(err, io.EOF) {
		return secretRead{value: "", err: fmt.Errorf("read secret input: %w", err)}
	}

	if len(data) > maximumSecretBytes {
		return secretRead{value: "", err: errSecret}
	}

	value := strings.TrimSuffix(strings.TrimSuffix(data, "\n"), "\r")
	if value == "" {
		return secretRead{value: "", err: errSecret}
	}

	return secretRead{value: value, err: nil}
}
