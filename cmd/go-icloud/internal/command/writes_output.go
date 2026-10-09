package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

var errWriteResult = errors.New("cannot save private write result")

// WriteResultError retains a private-output failure without displaying its path or data.
type WriteResultError struct{ Cause error }

// Error returns a credential-safe diagnostic.
func (failure *WriteResultError) Error() string { return errWriteResult.Error() }

// Unwrap permits explicit inspection of the underlying storage failure.
func (failure *WriteResultError) Unwrap() error { return failure.Cause }

func finishWriteResult(ctx context.Context, result any, path string) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, &WriteResultError{Cause: err}
	}
	if path != "" {
		data, err := json.Marshal(result)
		if err != nil {
			return nil, &WriteResultError{Cause: err}
		}
		if err = savePrivateData(ctx, path, data); err != nil {
			return nil, &WriteResultError{Cause: err}
		}
	}

	return safeWriteResult(result)
}

// Preserve semantic fields while excluding credentials and uninterpreted provider
// material. This projection is applied before the shared console encoder.
func safeWriteResult(result any) (any, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("encode write projection: %w", err)
	}

	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err = decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode write projection: %w", err)
	}

	stripWriteSecrets(value)

	return value, nil
}

func stripWriteSecrets(value any) {
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			if privateWriteField(key) {
				delete(node, key)
				continue
			}
			stripWriteSecrets(child)
		}
	case []any:
		for _, child := range node {
			stripWriteSecrets(child)
		}
	}
}

func privateWriteField(key string) bool {
	switch key {
	case "auth", "accountID", "clientID", "dsid", "cookies", "headers", "metadata", "responses",
		"assetMetadata", "masterMetadata", "versions", "dimensions", "size", "checksum",
		"fileAssetURL", "uploadURLs", "receipt", "wrappingKey", "uploadToken", "syncToken":
		return true
	default:
		return false
	}
}
