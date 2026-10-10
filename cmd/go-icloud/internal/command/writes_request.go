package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	errWriteRequest     = errors.New("cannot decode private write request")
	errNullWriteRequest = errors.New("write request must be a nonnull JSON object")
	errWriteTrailing    = errors.New("write request has multiple JSON values")
)

// WriteRequestError retains an input failure without displaying private data.
type WriteRequestError struct{ Cause error }

// Error returns a credential-safe diagnostic.
func (failure *WriteRequestError) Error() string { return errWriteRequest.Error() }

// Unwrap permits explicit inspection of the underlying file or JSON failure.
func (failure *WriteRequestError) Unwrap() error { return failure.Cause }

func readWriteRequest[Request any](ctx context.Context, path string) (*Request, error) {
	err := ctx.Err()
	if err != nil {
		return nil, &WriteRequestError{Cause: err}
	}

	file, err := openWriteFile(path)
	if err != nil {
		return nil, &WriteRequestError{Cause: err}
	}

	request, decodeErr := decodeWriteRequest[Request](file)
	closeErr := file.Close()
	if decodeErr != nil {
		return nil, &WriteRequestError{Cause: decodeErr}
	}

	if closeErr != nil {
		return nil, &WriteRequestError{Cause: closeErr}
	}

	return request, nil
}

func openWriteFile(path string) (*os.File, error) {
	directory, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("open request directory: %w", err)
	}

	defer func() { _ = directory.Close() }()

	file, err := directory.Open(filepath.Base(path))
	if err != nil {
		return nil, fmt.Errorf("open private input: %w", err)
	}

	return file, nil
}

func decodeWriteRequest[Request any](input io.Reader) (*Request, error) {
	decoder := json.NewDecoder(input)

	var raw json.RawMessage

	err := decoder.Decode(&raw)
	if err != nil {
		return nil, fmt.Errorf("decode write JSON: %w", err)
	}

	var trailing json.RawMessage

	err = decoder.Decode(&trailing)
	if !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, fmt.Errorf("decode request end: %w", err)
		}
		return nil, errWriteTrailing
	}

	err = validateWriteJSON(raw)
	if err != nil {
		return nil, err
	}
	typed := json.NewDecoder(bytes.NewReader(raw))
	typed.DisallowUnknownFields()

	var request *Request

	err = typed.Decode(&request)
	if err != nil {
		return nil, fmt.Errorf("decode typed request: %w", err)
	}
	if request == nil {
		return nil, errNullWriteRequest
	}

	return request, nil
}

func invokeWrite[Request, Result any](ctx context.Context, path, resultPath string,
	invoke func(Request) (*Result, error),
) (any, error) {
	request, err := readWriteRequest[Request](ctx, path)
	if err != nil {
		return nil, err
	}

	err = ctx.Err()
	if err != nil {
		return nil, &WriteRequestError{Cause: err}
	}

	result, err := invoke(*request)
	if err != nil {
		return nil, fmt.Errorf("SDK write: %w", err)
	}

	return finishWriteResult(ctx, result, resultPath)
}
