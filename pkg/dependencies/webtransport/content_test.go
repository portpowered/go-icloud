package webtransport_test

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
)

const decodedTestContent = "decoded"

const rawDeflateTestContent = "raw-deflate"

var errContentClose = errors.New("synthetic response close failure")

// These are synthetic wire controls; canonical Source replay bodies are already decoded.
func TestCompressedHTTPContent(t *testing.T) {
	t.Parallel()

	codings := []string{"gzip", "x-gzip", "deflate", rawDeflateTestContent, "gzip, deflate", decodedTestContent}
	for _, coding := range codings {
		t.Run(coding, func(t *testing.T) {
			t.Parallel()
			assertCompressedDiscovery(t, coding)
		})
	}
}

func assertCompressedDiscovery(t *testing.T, coding string) {
	t.Helper()

	plain := []byte(`{"content":[]}`)
	data := compressedTestContent(t, coding, plain)
	body := &trackedContentBody{Reader: bytes.NewReader(data), closed: false, closeErr: nil}
	response := new(http.Response)
	response.StatusCode = http.StatusOK
	response.Header = make(http.Header)
	response.Body = body
	response.Header.Set(protocol.HTTPContentEncodingName, coding)

	response.Uncompressed = coding == decodedTestContent
	if coding == rawDeflateTestContent {
		response.Header.Set(protocol.HTTPContentEncodingName, "deflate")
	}

	if coding == decodedTestContent {
		response.Header.Set(protocol.HTTPContentEncodingName, "gzip")
	}

	client := webtransport.New(findMyRoundTrip(func(_ *http.Request) (*http.Response, error) { return response, nil }))

	result, err := client.InitializeFindMy(t.Context(), findMyTestAuth(), false)
	if err != nil || result == nil || !body.closed {
		t.Fatalf("compressed discovery failed or leaked response body: %v", err)
	}
}

func compressedTestContent(t *testing.T, coding string, data []byte) []byte {
	t.Helper()

	if coding == decodedTestContent {
		return data
	}

	if coding == "gzip, deflate" {
		return compressedTestContent(t, "deflate", compressedTestContent(t, "gzip", data))
	}

	var output bytes.Buffer

	var writer io.WriteCloser

	var err error

	switch coding {
	case "gzip", "x-gzip":
		writer = gzip.NewWriter(&output)
	case "deflate":
		writer = zlib.NewWriter(&output)
	case rawDeflateTestContent:
		writer, err = flate.NewWriter(&output, flate.DefaultCompression)
	default:
		t.Fatal("unsupported test coding")
	}

	if err != nil {
		t.Fatal(err)
	}

	_, err = writer.Write(data)
	if err != nil {
		t.Fatal(err)
	}

	err = writer.Close()
	if err != nil {
		t.Fatal(err)
	}

	return output.Bytes()
}

type trackedContentBody struct {
	*bytes.Reader

	closed   bool
	closeErr error
}

func (body *trackedContentBody) Close() error {
	body.closed = true

	return body.closeErr
}

func TestCompressedResponseCloseFailurePreservesCause(t *testing.T) {
	t.Parallel()

	body := &trackedContentBody{Reader: bytes.NewReader(compressedTestContent(t, "gzip", []byte(`{"content":[]}`))),
		closed: false, closeErr: errContentClose}
	response := new(http.Response)
	response.StatusCode = http.StatusOK
	response.Header = make(http.Header)
	response.Body = body
	response.Header.Set(protocol.HTTPContentEncodingName, "gzip")

	client := webtransport.New(findMyRoundTrip(func(_ *http.Request) (*http.Response, error) { return response, nil }))

	_, err := client.InitializeFindMy(t.Context(), findMyTestAuth(), false)
	if !errors.Is(err, errContentClose) || !body.closed {
		t.Fatal("compressed response lost close failure or ownership")
	}
}
