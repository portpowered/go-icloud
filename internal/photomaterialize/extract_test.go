package photomaterialize_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/internal/photomaterialize"
)

func TestNormalizedAndInvalidMetadata(t *testing.T) {
	t.Parallel()

	metadata := photomaterialize.ExtractMetadataJSON(json.RawMessage(`{"fields":{` +
		`"assetDate":{"value":"2024-01-02T03:04:05-08:00"},"timeZoneOffset":{"value":3600},` +
		`"captionEnc":{"value":"\u5199\u771f"},"extendedDescEnc":{"value":null},` +
		`"isFavorite":{"value":true},"locationEnc":{"value":"invalid"},` +
		`"keywordsEnc":{"value":"invalid"},"adjustmentSimpleDataEnc":{"value":"invalid"}}}`))
	if metadata == nil || metadata.Title == nil || *metadata.Title != "\u5199\u771f" || metadata.Description != nil {
		t.Fatalf("normalized text or null metadata changed: %+v", metadata)
	}

	assertNormalizedDate(t, metadata)

	for _, input := range []string{"null", "[]", "not JSON"} {
		if photomaterialize.ExtractMetadataJSON(json.RawMessage(input)) != nil {
			t.Fatalf("invalid metadata accepted: %s", input)
		}
	}
}

func assertNormalizedDate(t *testing.T, metadata *photomaterialize.Metadata) {
	t.Helper()

	if metadata.CreateDate == nil || metadata.CreateDate.Format("-0700") != "-0800" ||
		metadata.Rating == nil || *metadata.Rating != 5 {
		t.Fatalf("normalized date or rating changed: %+v", metadata)
	}

	if metadata.GpsLatitude != nil || metadata.Keywords != nil || metadata.Orientation != nil {
		t.Fatalf("unreadable embedded metadata produced values: %+v", metadata)
	}
}

func TestEmptyXMPAndEscaping(t *testing.T) {
	t.Parallel()

	metadata := new(photomaterialize.Metadata)
	metadata.Toolkit = string(photomaterialize.SourceToolkit)

	output := string(photomaterialize.RenderXMP(*metadata))
	if !strings.Contains(output, `xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" />`) ||
		strings.Contains(output, "rdf:Description") {
		t.Fatalf("empty Source sidecar shape changed: %s", output)
	}

	metadata.Toolkit = `custom "toolkit" & test`
	if !strings.Contains(string(photomaterialize.RenderXMP(*metadata)), "&quot;toolkit&quot; &amp; test") {
		t.Fatal("toolkit attribute was not escaped")
	}
}
