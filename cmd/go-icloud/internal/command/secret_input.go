package command

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
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
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("secret input canceled: %w", err)
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

		return "", errors.Join(fmt.Errorf("secret input canceled: %w", ctx.Err()), closeErr)
	case read := <-result:
		return read.value, read.err
	}
}

func readSecretLine(input io.Reader) secretRead {
	data, err := bufio.NewReader(io.LimitReader(input, maximumSecretBytes+1)).ReadString('\n')
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
