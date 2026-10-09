package replay_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/tests/replay"
)

func portableMultipart(t *testing.T) replay.Exchange {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("fixtures", "synthetic", "http", "drive-upload-binary.json"))
	if err != nil {
		t.Fatal(err)
	}

	var fixture struct {
		Exchanges []replay.Exchange `json:"exchanges"`
	}

	err = json.Unmarshal(data, &fixture)
	if err != nil {
		t.Fatal(err)
	}

	for _, exchange := range fixture.Exchanges {
		if exchange.Request.Body.Encoding == "multipart" {
			return exchange
		}
	}

	t.Fatal("missing portable multipart exchange")

	return sampleExchange()
}

func multipartRequest(t *testing.T, exchange replay.Exchange, boundary string) *http.Request {
	t.Helper()

	var buffer bytes.Buffer

	writer := multipart.NewWriter(&buffer)

	err := writer.SetBoundary(boundary)
	if err != nil {
		t.Fatal(err)
	}

	for _, part := range exchange.Request.Body.Parts {
		writeMultipartPart(t, writer, part)
	}

	err = writer.Close()
	if err != nil {
		t.Fatal(err)
	}

	request, err := http.NewRequestWithContext(t.Context(), exchange.Request.Method,
		exchange.Request.Origin+exchange.Request.Path, bytes.NewReader(buffer.Bytes()))
	if err != nil {
		t.Fatal(err)
	}

	for _, header := range exchange.Request.Headers {
		if !strings.EqualFold(header[0], "Content-Type") && !strings.EqualFold(header[0], "Content-Length") {
			request.Header.Add(header[0], header[1])
		}
	}

	request.Header.Set("Content-Type", writer.FormDataContentType())

	return request
}

func writeMultipartPart(t *testing.T, writer *multipart.Writer, part replay.MultipartPart) {
	t.Helper()

	header := make(textproto.MIMEHeader)

	for _, pair := range part.Headers {
		header.Add(pair[0], pair[1])
	}

	var encoded string

	err := json.Unmarshal(part.Body.Value, &encoded)
	if err != nil {
		t.Fatal(err)
	}

	body, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}

	partWriter, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}

	_, err = partWriter.Write(body)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPortableMultipartMatchesGoWriterWithNewBoundary(t *testing.T) {
	t.Parallel()

	exchange := portableMultipart(t)
	transport := newTransport(t, exchange)
	request := multipartRequest(t, exchange, strings.Repeat("a", 32))

	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}

	consumeResponse(t, response)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func TestMultipartChangesRejectBeforeResponse(t *testing.T) {
	t.Parallel()

	for _, change := range []string{"boundary", "format", "bytes", "filename", "preamble", "epilogue",
		"extra_part", "interior_close", changeLength, "duplicate_type"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()

			exchange := portableMultipart(t)
			transport := newTransport(t, exchange)
			request := multipartRequest(t, exchange, strings.Repeat("a", 32))
			mutateMultipart(t, request, change)
			assertRejected(t, transport, request)

			if transport.AssertConsumed() == nil {
				t.Fatal("caught multipart rejection was forgotten")
			}
		})
	}
}

func mutateMultipart(t *testing.T, request *http.Request, change string) {
	t.Helper()

	body, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatal(err)
	}

	err = request.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	body = mutateMultipartBytes(body, change)
	request.Body = io.NopCloser(bytes.NewReader(body))
	request.ContentLength = int64(len(body))
	mutateMultipartHeaders(request, change)
}

func mutateMultipartBytes(body []byte, change string) []byte {
	switch change {
	case "bytes":
		return bytes.Replace(body, []byte("synthetic upload"), []byte("different bytes"), 1)
	case "filename":
		return bytes.Replace(body, []byte("synthetic.txt"), []byte("different.txt"), 1)
	case "preamble":
		return append([]byte("unrecorded\r\n"), body...)
	case "epilogue":
		return append(body, []byte("unrecorded")...)
	case "extra_part":
		return bytes.Replace(body, []byte("--\r\n"), []byte("\r\nExtra: value\r\n\r\nextra\r\n--"+
			strings.Repeat("a", 32)+"--\r\n"), 1)
	case "interior_close":
		return bytes.Replace(body, []byte("synthetic upload"), []byte("\r\n--"+strings.Repeat("a", 32)+"--\r\n"), 1)
	default:
		return body
	}
}

func mutateMultipartHeaders(request *http.Request, change string) {
	switch change {
	case "boundary":
		request.Header.Set("Content-Type", "multipart/form-data; boundary="+strings.Repeat("b", 32))
	case "format":
		request.Header.Set("Content-Type", "multipart/form-data; boundary=short")
	case changeLength:
		request.Header.Set("Content-Length", strconv.FormatInt(request.ContentLength+1, 10))
	case "duplicate_type":
		request.Header.Add("Content-Type", request.Header.Get("Content-Type"))
	}
}

func TestAllPortableExchangesInstantiateAndMultipartUsesGoWriter(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(filepath.Join("fixtures", "synthetic", "http", "*.json"))
	if err != nil {
		t.Fatal(err)
	}

	pairs := 0
	uploads := 0

	for _, path := range paths {
		exchanges := loadPortableExchanges(t, path)
		newTransport(t, exchanges...)
		pairs += len(exchanges)

		for _, exchange := range exchanges {
			if exchange.Request.Body.Encoding == "multipart" {
				checkPortableMultipart(t, exchange)

				uploads++
			}
		}
	}

	if len(paths) != 1057 || pairs != 2354 || uploads != 15 {
		t.Fatalf("portable HTTP counts changed: scenarios=%d pairs=%d uploads=%d", len(paths), pairs, uploads)
	}
}

func loadPortableExchanges(t *testing.T, path string) []replay.Exchange {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	var fixture struct {
		Exchanges []replay.Exchange `json:"exchanges"`
	}

	err = json.Unmarshal(data, &fixture)
	if err != nil {
		t.Fatal(err)
	}

	return fixture.Exchanges
}

func checkPortableMultipart(t *testing.T, exchange replay.Exchange) {
	t.Helper()

	transport := newTransport(t, exchange)
	request := multipartRequest(t, exchange, strings.Repeat("a", 32))

	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}

	if response.StatusCode != exchange.Response.Status {
		t.Fatal("paired upload response status changed")
	}

	consumeResponse(t, response)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}
