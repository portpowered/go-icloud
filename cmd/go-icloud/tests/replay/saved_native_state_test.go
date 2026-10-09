package replay_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func assertCompleteNativeSession(t *testing.T, raw map[string]json.RawMessage, actual icloud.ResumeSessionResult) {
	t.Helper()

	if _, native := raw["initial_state"]; !native {
		return // Imported-reference flows compare their full constructor-derived snapshot separately.
	}

	var (
		initial, outcome, state map[string]json.RawMessage
		exchanges               []replay.Exchange
	)

	decode(t, raw["initial_state"], &initial)
	decode(t, raw["result"], &outcome)
	decode(t, outcome["auth_state"], &state)
	decode(t, raw["exchanges"], &exchanges)
	row := map[string]json.RawMessage{
		"authState": outcome["auth_state"], "headers": initial["headers"],
		"cookieState": nativeSourceCookieState(t, state),
	}
	expected := expectedReferenceSession(t, row, "authState", exchanges[len(exchanges)-1])

	expected.Responses = make([]icloud.ResponseMetadata, 0, len(exchanges))
	for _, exchange := range exchanges {
		expected.Responses = append(expected.Responses, referenceResponseMetadata(exchange))
	}

	assertSavedSessionSnapshot(t, actual, expected)
}

func nativeSourceCookieState(t *testing.T, state map[string]json.RawMessage) json.RawMessage {
	t.Helper()

	var records []map[string]json.RawMessage

	decode(t, state["cookies"], &records)

	cookies := make([]referenceSavedCookie, 0, len(records))

	for _, record := range records {
		var (
			cookie     referenceSavedCookie
			attributes map[string]json.RawMessage
		)

		decode(t, mustEncodeNativeCookie(t, record), &cookie)
		decode(t, record["attributes"], &attributes)

		if !strings.HasPrefix(cookie.Domain, ".") {
			t.Fatal("native cookie snapshot needs explicit Source domain-specified evidence")
		}

		cookie.DomainSpecified = true
		_, cookie.HTTPOnly = attributes["HttpOnly"]
		cookies = append(cookies, cookie)
	}

	encoded, err := json.Marshal(cookies)
	if err != nil {
		t.Fatal(err)
	}

	return encoded
}

func mustEncodeNativeCookie(t *testing.T, record map[string]json.RawMessage) []byte {
	t.Helper()

	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}

	return encoded
}
