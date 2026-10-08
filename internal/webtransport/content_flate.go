package webtransport

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/adler32"
	"io"

	"github.com/dsnet/compress/flate"
	"github.com/portpowered/go-icloud/internal/protocol"
)

type inflatedContent struct {
	data     []byte
	consumed int
	complete bool
}

func inflateContent(data []byte) (inflatedContent, error) {
	var result inflatedContent

	reader, err := flate.NewReader(bytes.NewReader(data), nil)
	if err != nil {
		return result, fmt.Errorf("create HTTP DEFLATE decoder: %w", err)
	}

	decoded, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	result.data = decoded
	result.consumed = int(reader.InputOffset)
	result.complete = readErr == nil

	return result, errors.Join(incompleteContentError(readErr), incompleteContentError(closeErr))
}

func decodeDeflateContent(data []byte) ([]byte, error) {
	if len(data) < protocol.ZlibHeaderLength {
		return []byte{}, nil
	}

	if !zlibHeader(data) {
		result, err := inflateContent(data)

		return result.data, err
	}

	if data[1]&protocol.ZlibDictionaryFlag != 0 {
		if len(data) < protocol.ZlibHeaderLength+protocol.ContentChecksumBytes {
			return []byte{}, nil
		}

		return nil, zlib.ErrDictionary
	}

	result, err := inflateContent(data[protocol.ZlibHeaderLength:])
	if err != nil {
		return result.data, err
	}

	trailer := data[protocol.ZlibHeaderLength+result.consumed:]
	if result.complete && len(trailer) >= protocol.ContentChecksumBytes &&
		binary.BigEndian.Uint32(trailer) != adler32.Checksum(result.data) {
		return result.data, zlib.ErrChecksum
	}

	return result.data, nil
}

func zlibHeader(data []byte) bool {
	return data[0]&protocol.ZlibMethodMask == protocol.DeflateMethod &&
		data[0]>>protocol.ZlibMethodBits <= protocol.ZlibMaxWindow &&
		binary.BigEndian.Uint16(data[:protocol.ZlibHeaderLength])%protocol.ZlibHeaderCheck == 0
}
