package webtransport

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"hash/crc32"
	"math"

	"github.com/portpowered/go-icloud/internal/protocol"
)

func decodeGzipContent(data []byte) ([]byte, error) {
	output := []byte{}
	completed := false

	for len(data) != 0 {
		header, err := gzipHeader(data)
		if err != nil {
			return gzipMemberFailure(output, completed, err)
		}

		if header == 0 {
			return output, nil
		}

		result, err := inflateContent(data[header:])
		if err != nil {
			return gzipMemberFailure(output, completed, err)
		}

		trailer := data[header+result.consumed:]
		if result.complete {
			err = validateGzipTrailer(trailer, result.data)
			if err != nil {
				return gzipMemberFailure(output, completed, err)
			}
		}

		output = append(output, result.data...)
		if !result.complete || len(trailer) < protocol.GzipTrailerLength {
			return output, nil
		}

		completed = true
		data = trailer[protocol.GzipTrailerLength:]
	}

	return output, nil
}

func gzipMemberFailure(output []byte, completed bool, err error) ([]byte, error) {
	if completed {
		// urllib3 accepts trailing data after a completed member, including
		// a malformed subsequent member; first-member errors remain failures.
		return output, nil
	}

	return output, err
}

func validateGzipPrefix(data []byte) error {
	if len(data) >= 2 && (data[0] != protocol.GzipMagicFirst || data[1] != protocol.GzipMagicSecond) {
		return gzip.ErrHeader
	}

	if len(data) >= 4 && (data[2] != protocol.DeflateMethod || data[3]&protocol.GzipReservedMask != 0) {
		return gzip.ErrHeader
	}

	return nil
}

func gzipHeader(data []byte) (int, error) {
	err := validateGzipPrefix(data)
	if err != nil || len(data) < protocol.GzipHeaderLength {
		return 0, err
	}

	position := protocol.GzipHeaderLength
	if data[3]&protocol.GzipExtraFlag != 0 {
		if len(data)-position < protocol.ContentLengthBytes {
			return 0, nil
		}

		size := int(binary.LittleEndian.Uint16(data[position:]))

		position += protocol.ContentLengthBytes + size
		if position > len(data) {
			return 0, nil
		}
	}

	return gzipHeaderText(data, position)
}

func gzipHeaderText(data []byte, position int) (int, error) {
	for _, flag := range []byte{protocol.GzipFilenameFlag, protocol.GzipCommentFlag} {
		if data[3]&flag == 0 {
			continue
		}

		end := bytes.IndexByte(data[position:], 0)
		if end < 0 {
			return 0, nil
		}

		position += end + 1
	}

	if data[3]&protocol.GzipHeaderCRCFlag != 0 {
		if len(data)-position < protocol.ContentLengthBytes {
			return 0, nil
		}

		checksum := uint16(crc32.ChecksumIEEE(data[:position]) & math.MaxUint16)
		if binary.LittleEndian.Uint16(data[position:]) != checksum {
			return 0, gzip.ErrHeader
		}

		position += protocol.ContentLengthBytes
	}

	return position, nil
}

func validateGzipTrailer(trailer, data []byte) error {
	if len(trailer) >= protocol.ContentChecksumBytes && binary.LittleEndian.Uint32(trailer) != crc32.ChecksumIEEE(data) {
		return gzip.ErrChecksum
	}

	// RFC 1952 stores the uncompressed size modulo 2^32.
	size := uint32(uint64(len(data)) & math.MaxUint32)
	if len(trailer) >= protocol.GzipTrailerLength &&
		binary.LittleEndian.Uint32(trailer[protocol.ContentChecksumBytes:]) != size {
		return gzip.ErrChecksum
	}

	return nil
}
