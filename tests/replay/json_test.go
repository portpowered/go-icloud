package replay_test

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/tests/replay"
)

const (
	testJSONPattern = "json-pattern"
	patternBody     = `{"items":[{"id":"TempId-aa","flag":true}],"big":9007199254740993}`
	redactedBody    = `{"Password":"<redacted:nonempty-password>",` +
		`"items":[{"verificationCode":"<redacted:six-digit-code>"}],"fixed":true}`
	unknownRule = "unknown"
)

func patternEntity() replay.Entity {
	return replay.Entity{
		Encoding: testJSONPattern, Value: json.RawMessage(patternBody), ContentTypePattern: "", Parts: nil,
		Matchers: []replay.JSONMatcher{{
			Path:    []json.RawMessage{json.RawMessage(`"items"`), json.RawMessage(`0`), json.RawMessage(`"id"`)},
			Pattern: "TempId-[a-z]{2}",
		}},
	}
}

func jsonExchange(entity replay.Entity) replay.Exchange {
	exchange := sampleExchange()
	exchange.Request.Body = entity
	exchange.Request.Headers = exchange.Request.Headers[:1]

	return exchange
}

func runJSONRequest(t *testing.T, transport *replay.HTTPTransport, body string, success bool) {
	t.Helper()

	request := sampleRequest(t)
	request.Body = io.NopCloser(strings.NewReader(body))
	request.ContentLength = int64(len(body))

	response, err := transport.RoundTrip(request)
	if err == nil {
		consumeResponse(t, response)
	}

	if (err == nil) != success {
		t.Fatalf("wrong JSON rule disposition: success=%t err=%v", success, err)
	}

	consumed := transport.AssertConsumed()
	if success && consumed != nil {
		t.Fatal(consumed)
	}

	if !success && !errors.Is(consumed, replay.ErrMismatch) {
		t.Fatal("caught JSON rejection was forgotten")
	}
}

func TestJSONPatternBindsAllUndeclaredFieldsAndFullFormat(t *testing.T) {
	t.Parallel()

	for _, body := range []string{
		`{"big":9007199254740993,"items":[{"flag":true,"id":"TempId-bb"}]}`,
		strings.Replace(patternBody, replayLiteralTempIDAa, "prefix-TempId-aa", 1),
		strings.Replace(patternBody, replayLiteralTempIDAa, "TempId-aa-suffix", 1),
		strings.Replace(patternBody, "true", "false", 1),
		strings.Replace(patternBody, photoUploadOpaqueInteger, "9007199254740992", 1),
		strings.Replace(patternBody, `"TempId-aa"`, reminderChangeNullValue, 1),
		strings.Replace(patternBody, `"id":"TempId-aa"`, `"missing":"TempId-aa"`, 1),
		strings.Replace(patternBody, `"flag":true`, `"flag":true,"extra":1`, 1),
		patternBody + `{}`,
		strings.Replace(patternBody, `"flag":true`, `"flag":false,"flag":true`, 1),
	} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			transport := newTransport(t, jsonExchange(patternEntity()))
			runJSONRequest(t, transport, body, strings.HasPrefix(body, `{"big"`))
		})
	}
}

func TestJSONRedactionChecksNestedTypesAndCodeShape(t *testing.T) {
	t.Parallel()

	entity := replay.Entity{Encoding: replayLiteralJSONRedacted, Value: json.RawMessage(redactedBody),
		Matchers: nil, ContentTypePattern: "", Parts: nil,
	}
	valid := `{"Password":"invented","items":[{"verificationCode":"654321"}],"fixed":true}`

	for _, body := range []string{
		valid,
		strings.Replace(valid, "invented", "", 1),
		strings.Replace(valid, `"invented"`, "false", 1),
		strings.Replace(valid, "654321", "123", 1),
		strings.Replace(valid, "654321", "１２３４５６", 1),
		strings.Replace(valid, `"654321"`, "654321", 1),
		strings.Replace(valid, `"fixed":true`, `"fixed":false`, 1),
	} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			transport := newTransport(t, jsonExchange(entity))
			runJSONRequest(t, transport, body, body == valid)
		})
	}
}

func TestRedactedLengthIsVerifiedBeforeIgnoringRecordedSize(t *testing.T) {
	t.Parallel()

	entity := replay.Entity{Encoding: replayLiteralJSONRedacted, Value: json.RawMessage(redactedBody),
		Matchers: nil, ContentTypePattern: "", Parts: nil,
	}
	body := `{"Password":"invented","items":[{"verificationCode":"654321"}],"fixed":true}`

	for _, length := range []string{"1", "999"} {
		t.Run(length, func(t *testing.T) {
			t.Parallel()

			exchange := jsonExchange(entity)
			exchange.Request.Headers = append(exchange.Request.Headers, replay.Pair{contentLengthHeader, "999"})
			transport := newTransport(t, exchange)
			request := sampleRequest(t)
			request.Body = io.NopCloser(strings.NewReader(body))
			request.ContentLength = int64(len(body))
			request.Header.Set(replayLiteralContentLength, length)
			assertRejected(t, transport, request)
		})
	}

	exchange := jsonExchange(entity)
	exchange.Request.Headers = append(exchange.Request.Headers, replay.Pair{contentLengthHeader, "999"})
	runJSONRequest(t, newTransport(t, exchange), body, true)
}

func TestInvalidJSONRuleDeclarationsFailConstruction(t *testing.T) {
	t.Parallel()

	for _, change := range []string{
		replayLiteralNoMatchers, "path", "index", replayLiteralDuplicate, replayLiteralEscapedDuplicate,
		"regex", "sample", unknownRule} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()

			entity := patternEntity()
			changeMatcher(&entity, change)

			_, err := replay.NewHTTPTransport([]replay.Exchange{jsonExchange(entity)})
			if !errors.Is(err, replay.ErrFixture) {
				t.Fatalf("invalid matcher accepted: %v", err)
			}
		})
	}

	var entity replay.Entity

	err := json.Unmarshal([]byte(`{"encoding":"base64","value":"","ignore":true}`), &entity)
	if !errors.Is(err, replay.ErrFixture) {
		t.Fatalf("unknown rule field accepted: %v", err)
	}
}

func changeMatcher(entity *replay.Entity, change string) {
	switch change {
	case replayLiteralNoMatchers:
		entity.Matchers = nil
	case "path":
		entity.Matchers[0].Path = nil
	case "index":
		entity.Matchers[0].Path[1] = json.RawMessage(`5`)
	case replayLiteralDuplicate:
		entity.Matchers = append(entity.Matchers, entity.Matchers[0])
	case replayLiteralEscapedDuplicate:
		alias := replay.JSONMatcher{
			Path:    []json.RawMessage{json.RawMessage(`"items"`), json.RawMessage(`0`), json.RawMessage(`"\u0069d"`)},
			Pattern: replayLiteralTempIDAa,
		}
		entity.Matchers = append(entity.Matchers, alias)
	case "regex":
		entity.Matchers[0].Pattern = "["
	case "sample":
		entity.Value = json.RawMessage(`{"items":[{"id":false}]}`)
	case unknownRule:
		entity.Encoding = unknownRule
	}
}

func TestPortableJSONDeclarationsAreAccepted(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(filepath.Join(replayExpectedFixtures, replayExpectedSynthetic, "http", "*.json"))
	if err != nil {
		t.Fatal(err)
	}

	count := 0

	for _, path := range paths {
		data, readErr := os.ReadFile(filepath.Clean(path))
		if readErr != nil {
			t.Fatal(readErr)
		}

		var fixture struct {
			Exchanges []json.RawMessage `json:"exchanges"`
		}

		err = json.Unmarshal(data, &fixture)
		if err != nil {
			t.Fatal(err)
		}

		count += checkPortableJSON(t, fixture.Exchanges)
	}

	if count != 26 {
		t.Fatalf("portable JSON declaration count changed: %d", count)
	}
}

func checkPortableJSON(t *testing.T, pairs []json.RawMessage) int {
	t.Helper()

	count := 0

	for _, pair := range pairs {
		if !strings.Contains(string(pair), `"json-pattern"`) && !strings.Contains(string(pair), `"json-redacted"`) {
			continue
		}

		var exchange replay.Exchange

		err := json.Unmarshal(pair, &exchange)
		if err != nil {
			t.Fatal(err)
		}

		newTransport(t, exchange)

		count++
	}

	return count
}
