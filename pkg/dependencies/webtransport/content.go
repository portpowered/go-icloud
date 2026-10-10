package webtransport

import (
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/account"
)

func readResponseContent(response *http.Response) ([]byte, error) {
	data, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()

	if readErr != nil || response.Uncompressed {
		return data, errors.Join(readErr, closeErr)
	}

	decoded, decodeErr := decodeResponseContent(data, response.Header.Get(protocol.HTTPContentEncodingName))

	return decoded, errors.Join(decodeErr, closeErr)
}

func decodeResponseContent(data []byte, coding string) ([]byte, error) {
	modes := strings.Split(strings.ToLower(coding), ",")

	for _, mode := range slices.Backward(modes) {
		var err error

		switch strings.TrimSpace(mode) {
		case string(account.Gzip), string(account.XGzip):
			data, err = decodeGzipContent(data)
			if err != nil {
				return data, err
			}

			continue
		case string(account.Deflate):
			data, err = decodeDeflateContent(data)
		case string(account.Identity):
			continue
		default:
			continue
		}

		if err != nil {
			return data, err
		}
	}

	return data, nil
}

func incompleteContentError(err error) error {
	// The reference's incremental zlib decoder returns available output
	// without requiring a completed stream or trailer at HTTP EOF. This
	// policy applies only to owned decoders, never the HTTP response body.
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return nil
	}

	return err
}
