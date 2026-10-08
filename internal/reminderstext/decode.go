// Package reminderstext decodes the versioned CRDT documents used by Reminders.
package reminderstext

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"fmt"
	"io"

	"github.com/portpowered/go-icloud/internal/reminderstext/pb"
	"google.golang.org/protobuf/proto"
)

var errDocument = errors.New("unable to decode CRDT document")

// DecodeError identifies an unreadable provider document and retains its cause.
type DecodeError struct {
	// Cause preserves the compression or protobuf rejection.
	Cause error
}

// Error reports the reference-compatible document failure.
func (failure *DecodeError) Error() string {
	if errors.Is(failure.Cause, io.ErrUnexpectedEOF) {
		return "Compressed file ended before the end-of-stream marker was reached"
	}

	var corrupt flate.CorruptInputError

	if errors.As(failure.Cause, &corrupt) {
		return failure.Cause.Error()
	}

	return "Unable to decode CRDT document"
}

// ReferenceClass identifies the source exception family independently of native
// Go compression messages. Corrupt DEFLATE retains the native Go cause/message.
func (failure *DecodeError) ReferenceClass() string {
	if errors.Is(failure.Cause, io.ErrUnexpectedEOF) {
		return "EOFError"
	}

	var corrupt flate.CorruptInputError

	if errors.As(failure.Cause, &corrupt) {
		return "error"
	}

	return "CRDTDecodeError"
}

// Unwrap exposes the protobuf failure when one was available.
func (failure *DecodeError) Unwrap() error { return failure.Cause }

// Decode reads raw CloudKit bytes, accepting zlib, gzip and uncompressed documents.
// It tries the current envelope, a legacy version and a bare nonempty string in
// reference order. An envelope may legitimately contain an empty title or notes.
// The returned string preserves raw text bytes, including invalid UTF-8; the
// domain adapter owns the reference mapper's replacement-character policy.
func Decode(data []byte) (string, error) {
	data, err := decompress(data)
	if err != nil {
		return "", &DecodeError{Cause: err}
	}

	return decodeDocument(data)
}

func decodeDocument(data []byte) (string, error) {
	var document pb.Document

	err := proto.Unmarshal(data, &document)
	if err == nil && len(document.GetVersion()) != 0 {
		text, textErr := decodeString(document.GetVersion()[0].GetData())
		if textErr == nil {
			return text, nil
		}
	}

	var version pb.Version

	err = proto.Unmarshal(data, &version)

	if err == nil && len(version.GetData()) != 0 {
		text, textErr := decodeString(version.GetData())
		if textErr == nil {
			return text, nil
		}
	}

	text, err := decodeString(data)
	if err == nil && text != "" {
		return text, nil
	}

	if err == nil {
		err = errDocument
	}

	return "", &DecodeError{Cause: err}
}

func decodeString(data []byte) (string, error) {
	var value pb.String

	err := proto.Unmarshal(data, &value)
	if err != nil {
		return "", fmt.Errorf("parse Reminders text: %w", err)
	}

	return value.GetString_(), nil
}

func decompress(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err == nil {
		decoded, decodeErr := readCompressed(reader)
		if decodeErr == nil {
			return decoded, nil
		}
	}

	return decompressGzip(data)
}

func decompressGzip(original []byte) ([]byte, error) {
	remaining := original

	var decoded []byte

	for len(remaining) != 0 {
		if !bytes.HasPrefix(remaining, []byte("\x1f\x8b")) {
			return original, nil
		}

		input := bytes.NewReader(remaining)

		reader, err := gzip.NewReader(input)
		if err != nil {
			return gzipFallback(original, err)
		}

		reader.Multistream(false)

		member, err := readCompressed(reader)
		if err != nil {
			return gzipFallback(original, err)
		}

		decoded = append(decoded, member...)
		remaining = bytes.TrimLeft(remaining[len(remaining)-input.Len():], "\x00")
	}

	return decoded, nil
}

func gzipFallback(original []byte, err error) ([]byte, error) {
	if errors.Is(err, gzip.ErrHeader) || errors.Is(err, gzip.ErrChecksum) || errors.Is(err, io.EOF) {
		return original, nil
	}

	return nil, fmt.Errorf("decompress gzip Reminders document: %w", err)
}

func readCompressed(reader io.ReadCloser) ([]byte, error) {
	data, err := io.ReadAll(reader)

	err = errors.Join(err, reader.Close())
	if err != nil {
		return nil, fmt.Errorf("decompress Reminders text: %w", err)
	}

	return data, nil
}
