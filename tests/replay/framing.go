package replay

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

const (
	transferEncodingHeader = "transfer-encoding"
	declaredChunked        = "chunked"
)

func framingValues(headers []Pair, name string) []string {
	var result []string

	for _, pair := range headers {
		if strings.EqualFold(pair[0], name) {
			result = append(result, pair[1])
		}
	}

	return result
}

func validateRequestFraming(headers []Pair) error {
	values := framingValues(headers, transferEncodingHeader)
	if len(values) == 0 {
		return nil
	}

	if !reflect.DeepEqual(values, []string{declaredChunked}) || len(framingValues(headers, "content-length")) != 0 {
		return fmt.Errorf("%w: unsupported request transfer encoding", ErrFixture)
	}

	return nil
}

func matchRequestFraming(request *http.Request, headers []Pair, length int) error {
	declared := framingValues(headers, transferEncodingHeader)
	if len(declared) == 0 {
		if len(request.TransferEncoding) != 0 || request.ContentLength != int64(length) {
			return fmt.Errorf("%w: content length or undeclared transfer encoding", ErrMismatch)
		}

		return nil
	}

	if request.ContentLength != -1 || !reflect.DeepEqual(request.TransferEncoding, declared) {
		return fmt.Errorf("%w: declared chunked framing", ErrMismatch)
	}

	for name := range request.Header {
		if strings.EqualFold(name, "content-length") {
			return fmt.Errorf("%w: chunked content length header", ErrMismatch)
		}
	}

	return nil
}

func addDeclaredFramingHeaders(request *http.Request, actual, expected map[string][]string) {
	// Go stores framing separately from Header. Reconstruct only explicitly
	// recorded fields; unrelated headers remain subject to the complete match.
	if _, required := expected["content-length"]; required && actual["content-length"] == nil {
		actual["content-length"] = []string{strconv.FormatInt(request.ContentLength, 10)}
	}

	if _, required := expected[transferEncodingHeader]; required && actual[transferEncodingHeader] == nil {
		actual[transferEncodingHeader] = request.TransferEncoding
	}
}
