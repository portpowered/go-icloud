package replay_test

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

type contentCorpus struct {
	Cases []contentCase `json:"cases"`
}

type contentCase struct {
	Name     string           `json:"name"`
	Exchange replay.Exchange  `json:"exchange"`
	Decoded  *string          `json:"decoded"`
	Error    *json.RawMessage `json:"error"`
}

func TestSourceCompressedWireResponses(t *testing.T) {
	t.Parallel()
	raw := authReplayObject(t, "fixtures/synthetic/binary/http-content.json")

	var corpus contentCorpus

	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	authReplayDecode(t, encoded, &corpus)

	if len(corpus.Cases) != 604 {
		t.Fatal("compressed wire corpus changed")
	}

	for _, item := range corpus.Cases {
		t.Run(item.Name, func(t *testing.T) { t.Parallel(); replayCompressedContent(t, item) })
	}
}

func replayCompressedContent(t *testing.T, item contentCase) {
	t.Helper()
	raw := authReplayObject(t, "fixtures/synthetic/http/auth-authenticate-cached.json")
	request := authReplayRequest(t, raw)
	request.Auth.Headers = append(request.Auth.Headers, icloud.Header{Name: "Accept-Encoding", Value: "gzip, deflate"})

	transport, err := replay.NewHTTPTransport([]replay.Exchange{item.Exchange})
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	result, callErr := client.ResumeSession(t.Context(), request)
	assertCompressedOutcome(t, item, result, callErr)

	if result != nil {
		encoded, encodeErr := json.Marshal([]replay.Exchange{item.Exchange})
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}

		assertAuthResponses(t, map[string]json.RawMessage{replayExpectedExchanges: encoded}, result)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func assertCompressedOutcome(t *testing.T, item contentCase, result *icloud.ResumeSessionResult, err error) {
	t.Helper()

	if item.Error != nil {
		assertCompressionFailure(t, err)
		assertCompressionCause(t, item.Name, err)

		return
	}

	if item.Decoded == nil {
		t.Fatal("compressed fixture has no expected outcome")
	}

	decoded, decodeErr := base64.StdEncoding.Strict().DecodeString(*item.Decoded)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}

	if !json.Valid(decoded) {
		assertDecodedInvalidJSON(t, err, decoded)

		return
	}

	if err != nil || result == nil || !bytes.Equal(result.AccountData, decoded) {
		t.Fatalf("decoded account response changed: %v", err)
	}
}

func assertDecodedInvalidJSON(t *testing.T, err error, decoded []byte) {
	t.Helper()

	var failure *icloud.ClientError
	if !errors.As(err, &failure) || failure.Kind() != icloud.InvalidResponse ||
		!bytes.Equal(failure.ResponseBody(), decoded) {
		if failure != nil {
			t.Logf("decoded bytes: got %d %q; want %d %q", len(failure.ResponseBody()), failure.ResponseBody(),
				len(decoded), decoded)
		}

		t.Fatalf("decoded invalid JSON changed: %v", err)
	}
}

func assertCompressionFailure(t *testing.T, err error) {
	t.Helper()

	var failure *icloud.ClientError
	if !errors.As(err, &failure) || failure.Kind() != icloud.Transport || failure.StatusCode() != 200 {
		t.Fatalf("compression error changed: %v", err)
	}
}

type corruptedContentError interface {
	error
	IsCorrupted() bool
}

func assertCompressionCause(t *testing.T, name string, err error) {
	t.Helper()

	causes := map[string]error{
		"gzip-corrupt-checksum":        gzip.ErrChecksum,
		"gzip-corrupt-size":            gzip.ErrChecksum,
		"zlib-corrupt-checksum":        zlib.ErrChecksum,
		"gzip-invalid-header":          gzip.ErrHeader,
		"gzip-header-checksum-invalid": gzip.ErrHeader,
	}

	expected, exists := causes[name]
	if exists && !errors.Is(err, expected) {
		t.Fatal("compression lost native checksum or header cause")
	}

	if name == "deflate-invalid-data" {
		var cause corruptedContentError
		if !errors.As(err, &cause) || !cause.IsCorrupted() {
			t.Fatal("compression lost native decoder cause")
		}
	}
}
