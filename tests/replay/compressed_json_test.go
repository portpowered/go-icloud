package replay_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/internal/reminderstext"
	"github.com/portpowered/go-icloud/tests/replay"
)

const sourceEmptyDocument = "eJzjYBAK4GAQYJDyEmKQEuBiAbGBPDCtwSglxsUBZP0HAn6gKJytJMMl" +
	"xSVwJfvUE+n2tIO9pe8fx0x0PSzExMEAxIwAhRkUpg=="

func TestCompressedJSONBindsCompleteDecodedBytes(t *testing.T) {
	t.Parallel()

	for _, change := range []string{
		"valid", replayLiteralDifferentDocument, replayLiteralChecksum,
		replayLiteralTrailing, "fixed", replayLiteralBase64Whitespace,
	} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()

			entity := replay.Entity{Encoding: testJSONPattern, Value: json.RawMessage(compressedBody(sourceEmptyDocument)),
				Matchers: []replay.JSONMatcher{{Path: []json.RawMessage{json.RawMessage(`"document"`)},
					Pattern: compressedJSONRule}}, Parts: nil, ContentTypePattern: ""}
			exchange := jsonExchange(entity)
			exchange.Request.Headers = append(exchange.Request.Headers,
				replay.Pair{contentLengthHeader, strconv.Itoa(len(entity.Value))})
			transport := newTransport(t, exchange)
			actual := changedCompressedBody(t, change)
			runJSONRequest(t, transport, actual, change == "valid")
		})
	}
}

func compressedBody(encoded string) string {
	return fmt.Sprintf(`{"document":%q,"fixed":true}`, encoded)
}

func changedCompressedBody(t *testing.T, change string) string {
	t.Helper()

	text := ""
	if change == replayLiteralDifferentDocument {
		text = "x"
	}

	encoded, err := reminderstext.Encode(text)
	if err != nil {
		t.Fatal(err)
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}

	switch change {
	case replayLiteralChecksum:
		data[len(data)-1] ^= 1
	case replayLiteralTrailing:
		data = append(data, 'x')
	}

	encoded = base64.StdEncoding.EncodeToString(data)
	if change == replayLiteralBase64Whitespace {
		encoded += "\n"
	}

	body := compressedBody(encoded)
	if change == "fixed" {
		body = strings.ReplaceAll(body, "true", "false")
	}

	return body
}
