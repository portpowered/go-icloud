package replay_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/portpowered/go-icloud/tests/replay"
)

const chunkedHeader = "transfer-encoding"

func chunkedExchange() replay.Exchange {
	exchange := sampleExchange()
	exchange.Request.Headers[1] = replay.Pair{chunkedHeader, chunkedEncoding}
	return exchange
}

func chunkedRequest(t *testing.T) *http.Request {
	t.Helper()
	request := sampleRequest(t)
	request.ContentLength = -1
	request.TransferEncoding = []string{chunkedEncoding}
	return request
}

func TestDeclaredChunkedReplay(t *testing.T) {
	t.Parallel()

	for _, empty := range []bool{false, true} {
		t.Run(stringName(empty), func(t *testing.T) {
			t.Parallel()
			exchange := chunkedExchange()
			request := chunkedRequest(t)
			if empty {
				exchange.Request.Body.Value = json.RawMessage(`""`)
				request.Body = io.NopCloser(bytes.NewReader(nil))
			}
			transport := newTransport(t, exchange)
			response, err := transport.RoundTrip(request)
			if err != nil {
				t.Fatal(err)
			}

			consumeResponse(t, response)
			err = transport.AssertConsumed()
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func stringName(empty bool) string {
	if empty {
		return "empty"
	}
	return "binary"
}

func TestDeclaredChunkedRejectsChangedFraming(t *testing.T) {
	t.Parallel()
	changes := map[string]func(*http.Request){
		"missing":     func(request *http.Request) { request.TransferEncoding = nil },
		"unsupported": func(request *http.Request) { request.TransferEncoding = []string{"gzip"} },
		"multiple": func(request *http.Request) {
			request.TransferEncoding = []string{chunkedEncoding, chunkedEncoding}
		},
		"known-length":    func(request *http.Request) { request.ContentLength = 3 },
		"length-header":   func(request *http.Request) { request.Header.Set("Content-Length", "3") },
		"wrong-body":      func(request *http.Request) { request.Body = io.NopCloser(bytes.NewReader([]byte("bad"))) },
		"chunked-trailer": func(request *http.Request) { request.Trailer = http.Header{"Unexpected": []string{"value"}} },
		"header-conflict": func(request *http.Request) { request.Header.Set("Transfer-Encoding", "gzip") },
	}

	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			transport := newTransport(t, chunkedExchange())
			request := chunkedRequest(t)
			change(request)
			assertRejected(t, transport, request)
			if !errors.Is(transport.AssertConsumed(), replay.ErrMismatch) {
				t.Fatal("caught framing rejection was forgotten")
			}
		})
	}
}

func TestInvalidChunkedDeclarationsFailBeforeTraffic(t *testing.T) {
	t.Parallel()

	for _, headers := range [][]replay.Pair{
		{{chunkedHeader, "gzip"}},
		{{chunkedHeader, chunkedEncoding}, {chunkedHeader, chunkedEncoding}},
		{{chunkedHeader, chunkedEncoding}, {contentLengthHeader, "3"}},
	} {
		exchange := chunkedExchange()
		exchange.Request.Headers = headers
		_, err := replay.NewHTTPTransport([]replay.Exchange{exchange})
		if !errors.Is(err, replay.ErrFixture) {
			t.Fatal("invalid framing declaration was accepted", err)
		}
	}
}
