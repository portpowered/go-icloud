package replay_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/portpowered/go-icloud/tests/replay"
)

type closeProbe struct {
	io.ReadCloser

	closes int
	fail   bool
}

func (probe *closeProbe) Close() error {
	probe.closes++
	if probe.fail {
		return replay.ErrFixture
	}

	err := probe.ReadCloser.Close()
	if err != nil {
		return fmt.Errorf("close test body: %w", err)
	}

	return nil
}

func TestRequestBodiesCloseOnEveryExit(t *testing.T) {
	t.Parallel()

	for _, change := range []string{"success", changeMethod, extraQuery, unexpectedRequest, "sticky", closeFailure} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()

			transport := newTransport(t, sampleExchange())
			prepareCloseProbe(t, transport, change)
		})
	}
}

func prepareCloseProbe(t *testing.T, transport *replay.HTTPTransport, change string) {
	t.Helper()

	if change == unexpectedRequest {
		transport = newTransport(t)
	}

	if change == "sticky" {
		wrong := sampleRequest(t)
		wrong.Method = http.MethodDelete
		assertRejected(t, transport, wrong)
	}

	request := sampleRequest(t)
	mutateRequest(request, change)
	probe := &closeProbe{ReadCloser: request.Body, closes: 0, fail: change == closeFailure}
	request.Body = probe

	response, err := transport.RoundTrip(request)
	if err == nil {
		consumeResponse(t, response)
	}

	if probe.closes != 1 || (err == nil) != (change == "success") {
		t.Fatalf("request ownership failed: closes=%d err=%v", probe.closes, err)
	}

	if change == closeFailure && !errors.Is(transport.AssertConsumed(), replay.ErrMismatch) {
		t.Fatal("caught close failure was forgotten")
	}
}

const (
	testBase64Encoding  = "base64"
	changeOrigin        = "origin"
	chunkedEncoding     = "chunked"
	changeMethod        = "method"
	unexpectedRequest   = "unexpected"
	contentLengthHeader = "content-length"
	changeLength        = "length"
	closeFailure        = "close_error"
	extraQuery          = "extra_query"
)

func sampleExchange() replay.Exchange {
	return replay.Exchange{
		Request: replay.Request{
			Method: http.MethodPost, Origin: "https://example.invalid", Path: "/files/a%2Fb",
			Query:   []replay.Pair{{"tag", "one"}, {"tag", "two"}, {"empty", ""}},
			Headers: []replay.Pair{{"accept", "application/octet-stream"}, {contentLengthHeader, "3"}},
			Body: replay.Entity{Encoding: testBase64Encoding, Value: json.RawMessage(`"AP9B"`),
				Matchers: nil, ContentTypePattern: "", Parts: nil,
			},
		},
		Response: &replay.Response{
			BodyRepresentation: "",
			Status:             http.StatusOK,
			Headers:            []replay.Pair{{"Set-Cookie", "first=one; Secure"}, {"Set-Cookie", "second=two; Secure"}},
			Body: replay.Entity{Encoding: testBase64Encoding, Value: json.RawMessage(`"AP9C"`),
				Matchers: nil, ContentTypePattern: "", Parts: nil,
			},
		},
		Error: "",
	}
}

func sampleRequest(t *testing.T) *http.Request {
	t.Helper()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		"https://example.invalid/files/a%2Fb?tag=one&tag=two&empty=", bytes.NewReader([]byte{0, 255, 'A'}))
	if err != nil {
		t.Fatal(err)
	}

	request.Header.Set("Accept", "application/octet-stream")

	return request
}

func newTransport(t *testing.T, exchanges ...replay.Exchange) *replay.HTTPTransport {
	t.Helper()

	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}

	return transport
}

func consumeResponse(t *testing.T, response *http.Response) []byte {
	t.Helper()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}

	err = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	return body
}

func TestExactBinaryResponseAndOwnership(t *testing.T) {
	t.Parallel()

	exchange := sampleExchange()
	transport := newTransport(t, exchange)
	exchange.Request.Headers[0][1] = "changed"
	exchange.Response.Headers[0][1] = "changed"

	response, err := transport.RoundTrip(sampleRequest(t))
	if err != nil {
		t.Fatal(err)
	}

	if response.StatusCode != http.StatusOK || len(response.Cookies()) != 2 ||
		!bytes.Equal(consumeResponse(t, response), []byte{0, 255, 'B'}) {
		t.Fatal("response status, cookies or exact binary body changed")
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func TestRequestMutationsStayRejected(t *testing.T) {
	t.Parallel()

	for _, change := range []string{changeMethod, changeOrigin, "escaped_path", "query_order", extraQuery,
		"header", "extra_header", "body", changeLength, "header_length", "host", "userinfo", chunkedEncoding, "fragment",
		"opaque", "force_query", "trailer", "unknown_length", "connection_close"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()

			transport := newTransport(t, sampleExchange())
			request := sampleRequest(t)
			mutateRequest(request, change)

			assertRejected(t, transport, request)
			assertRejected(t, transport, sampleRequest(t))

			if !errors.Is(transport.AssertConsumed(), replay.ErrMismatch) {
				t.Fatal("caught mismatch was forgotten")
			}
		})
	}
}

func assertRejected(t *testing.T, transport *replay.HTTPTransport, request *http.Request) {
	t.Helper()

	response, err := transport.RoundTrip(request)
	if err == nil {
		consumeResponse(t, response)
		t.Fatal("unexpected response from rejected traffic")
	}

	if !errors.Is(err, replay.ErrMismatch) {
		t.Fatalf("expected sticky mismatch: %v", err)
	}
}

func mutateRequest(request *http.Request, change string) {
	switch change {
	case changeMethod:
		request.Method = http.MethodPut
	case changeOrigin:
		request.URL.Host = "other.invalid"
	case "escaped_path":
		request.URL.RawPath = ""
	case "query_order":
		request.URL.RawQuery = "tag=two&tag=one&empty="
	case extraQuery:
		request.URL.RawQuery += "&extra=value"
	case "host":
		request.Host = "other.invalid"
	case "userinfo":
		request.URL.User = url.User("invented")
	case "fragment":
		request.URL.Fragment = unexpectedRequest
	default:
		mutateRequestFraming(request, change)
	}
}

func mutateRequestFraming(request *http.Request, change string) {
	switch change {
	case "opaque":
		request.URL.Opaque = "//other.invalid/wrong"
	case "force_query":
		request.URL.ForceQuery = true
	case "trailer":
		request.Trailer = http.Header{"Extra": []string{"value"}}
	case "unknown_length":
		request.ContentLength = -1
	case "connection_close":
		request.Close = true
	default:
		mutateRequestEntity(request, change)
	}
}

func mutateRequestEntity(request *http.Request, change string) {
	switch change {
	case "header":
		request.Header.Set("Accept", "wrong")
	case "extra_header":
		request.Header.Set("Authorization", "invented")
	case "body":
		request.Body = io.NopCloser(strings.NewReader("bad"))
	case changeLength:
		request.ContentLength++
	case "header_length":
		request.Header.Set("Content-Length", "003")
	case chunkedEncoding:
		request.TransferEncoding = []string{chunkedEncoding}
	}
}

func TestAllExchangesAndBodiesMustBeConsumed(t *testing.T) {
	t.Parallel()

	transport := newTransport(t, sampleExchange())
	if !errors.Is(transport.AssertConsumed(), replay.ErrUnconsumed) {
		t.Fatal("unrequested exchange accepted")
	}

	response, err := transport.RoundTrip(sampleRequest(t))
	if err != nil {
		t.Fatal(err)
	}

	err = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	if !errors.Is(transport.AssertConsumed(), replay.ErrUnconsumed) {
		t.Fatal("discarded unread body accepted")
	}

	transport = newTransport(t, sampleExchange())

	response, err = transport.RoundTrip(sampleRequest(t))
	if err != nil {
		t.Fatal(err)
	}

	_, err = io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}

	if !errors.Is(transport.AssertConsumed(), replay.ErrUnconsumed) {
		t.Fatal("unclosed body accepted")
	}

	consumeResponse(t, response)

	assertRejected(t, transport, sampleRequest(t))

	if !errors.Is(transport.AssertConsumed(), replay.ErrMismatch) {
		t.Fatal("duplicate request accepted")
	}
}

func TestExpectedTransportFailureConsumesItsExchange(t *testing.T) {
	t.Parallel()

	exchange := sampleExchange()
	exchange.Response = nil
	exchange.Error = "ReadTimeout"
	transport := newTransport(t, exchange)

	response, err := transport.RoundTrip(sampleRequest(t))
	if err == nil {
		consumeResponse(t, response)
		t.Fatal("transport failure exposed a response")
	}

	var failure replay.TransportError

	if response != nil || !errors.As(err, &failure) || failure.Class != "ReadTimeout" {
		t.Fatalf("recorded failure lost: %v", err)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func TestMalformedOrUnsupportedOutcomesFailBeforeReplay(t *testing.T) {
	t.Parallel()

	changes := []string{"encoding", "base64", "outcome", "no_outcome", "failure", "status", changeOrigin, "representation"}
	for _, change := range changes {
		t.Run(change, func(t *testing.T) {
			t.Parallel()

			exchange := sampleExchange()
			mutateExchange(&exchange, change)

			_, err := replay.NewHTTPTransport([]replay.Exchange{exchange})
			if !errors.Is(err, replay.ErrFixture) {
				t.Fatalf("invalid fixture accepted: %v", err)
			}
		})
	}
}

func mutateExchange(exchange *replay.Exchange, change string) {
	switch change {
	case "encoding":
		exchange.Request.Body.Encoding = testJSONPattern
	case "base64":
		exchange.Response.Body.Value = json.RawMessage(`"!"`)
	case "outcome":
		exchange.Error = "Timeout"
	case "no_outcome":
		exchange.Response = nil
	case "failure":
		exchange.Response = nil
		exchange.Error = "UnknownFailure"
	case "status":
		exchange.Response.Status = 0
	case "representation":
		exchange.Response.BodyRepresentation = "unknown"
	case changeOrigin:
		exchange.Request.Origin += "/path"
	}
}

func TestResponseCloseAndInspectionAreRaceSafe(t *testing.T) {
	t.Parallel()

	transport := newTransport(t, sampleExchange())

	response, err := transport.RoundTrip(sampleRequest(t))
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		closeErr := response.Body.Close()
		if closeErr != nil {
			t.Error(closeErr)
		}
	}()

	consumeResponse(t, response)

	err = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	var group sync.WaitGroup

	for range 10 {
		group.Add(1)

		go func() {
			defer group.Done()

			closeAndInspect(t, response.Body, transport)
		}()
	}

	group.Wait()
}

func closeAndInspect(t *testing.T, closer io.Closer, transport *replay.HTTPTransport) {
	t.Helper()

	closeErr := closer.Close()
	if closeErr != nil {
		t.Error(closeErr)
	}

	consumedErr := transport.AssertConsumed()
	if consumedErr != nil {
		t.Error(consumedErr)
	}
}

func TestPortableAccountExchangeUsesGoHTTPClient(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("fixtures", "synthetic", "http", "account-devices-one.json"))
	if err != nil {
		t.Fatal(err)
	}

	var fixture struct {
		Exchanges []replay.Exchange `json:"exchanges"`
		Result    json.RawMessage   `json:"result"`
	}

	err = json.Unmarshal(data, &fixture)
	if err != nil {
		t.Fatal(err)
	}

	transport := newTransport(t, fixture.Exchanges...)
	client := &http.Client{Transport: transport, CheckRedirect: nil, Jar: nil, Timeout: 0}

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		"https://drive.example.invalid/setup/web/device/getDevices?clientId=synthetic-client&dsid=synthetic-account", nil)
	if err != nil {
		t.Fatal(err)
	}

	request.Header.Set("Accept", "application/json")

	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}

	got := consumeResponse(t, response)

	var encoded string

	err = json.Unmarshal(fixture.Exchanges[0].Response.Body.Value, &encoded)
	if err != nil {
		t.Fatal(err)
	}

	want, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("portable fixture response bytes changed")
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}
