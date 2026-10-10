package replay_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const syntheticSharedPhotosOrigin = "https://shared.example.invalid"
const sharedFilenameTextRule = "filename-text"
const sharedEmptyLikedRule = "empty-liked"

const sharedInvalidDateRule = "invalid-date"
const sharedFractionalSizeRule = "fractional-size"
const sharedStringSizeRule = "string-size"
const sharedTruthyLikedRule = "truthy-liked"
const sharedNullLikeCountRule = "null-likecount"
const sharedStringLikeCountRule = "string-likecount"
const sharedMissingWidthRule = "missing-width"
const sharedMissingHeightRule = "missing-height"
const sharedIgnoredRecordsRule = "ignored-records"
const sharedScalarRecordsRule = "scalar-records"
const sharedMalformedMasterRefRule = "malformed-master-ref"

func TestSharedPhotosLegacyValueRules(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{sharedFilenameTextRule, sharedInvalidDateRule, sharedFractionalSizeRule, sharedStringSizeRule,
		sharedTruthyLikedRule, sharedEmptyLikedRule, sharedNullLikeCountRule, sharedStringLikeCountRule, sharedMissingWidthRule, sharedMissingHeightRule,
		sharedIgnoredRecordsRule, sharedScalarRecordsRule, sharedMalformedMasterRefRule} {
		t.Run(mutation, func(t *testing.T) { t.Parallel(); runSharedValueRule(t, mutation) })
	}
}

func runSharedValueRule(t *testing.T, mutation string) {
	t.Helper()
	scenario := readAccountScenario(t, "fixtures/synthetic/http/photos-upload-shared-get-found.json")
	response := scenario.Exchanges[2].Response
	var envelope map[string]json.RawMessage
	var records []map[string]json.RawMessage
	var fields map[string]json.RawMessage
	authReplayDecode(t, contractAuthBody(t, response.Body), &envelope)
	authReplayDecode(t, envelope["records"], &records)
	authReplayDecode(t, records[0]["fields"], &fields)
	mutateSharedRecordValues(mutation, fields, records)
	records[0]["fields"] = sharedValueJSON(t, fields)
	envelope["records"] = sharedValueJSON(t, records)
	if mutation == sharedIgnoredRecordsRule {
		ignored := json.RawMessage(`[null,42,"ignored",{"recordType":"future"},{"recordType":"CPLAsset","fields":42},`)
		envelope["records"] = append(ignored, envelope["records"][1:]...)
	}
	if mutation == sharedScalarRecordsRule {
		envelope["records"] = json.RawMessage(`42`)
	}
	response.Body.Value = sharedValueJSON(t, base64.StdEncoding.EncodeToString(sharedValueJSON(t, envelope)))
	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}
	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}
	auth := sdkAccountAuth(scenario.Initial)
	auth.PhotosServiceURL = scenario.Initial.Origin
	auth.SharedPhotosServiceURL = syntheticSharedPhotosOrigin
	result, err := client.GetSharedPhoto(t.Context(), icloud.GetSharedPhotoRequest{
		Auth: auth, Album: "synthetic-stream-0", PhotoID: "synthetic-asset-0"})
	if mutation == sharedMissingWidthRule || mutation == sharedMissingHeightRule ||
		mutation == sharedNullLikeCountRule || mutation == sharedStringLikeCountRule {
		if err == nil || result != nil {
			t.Fatal("missing dimensions and malformed typed counts must fail projection")
		}
		var failure *icloud.ClientError
		if !errors.As(err, &failure) || failure.Kind() != icloud.InvalidResponse {
			t.Fatal("projection failure must be typed InvalidResponse", err)
		}
		if consumeErr := transport.AssertConsumed(); consumeErr != nil {
			t.Fatal(consumeErr)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if mutation == sharedScalarRecordsRule || mutation == sharedMalformedMasterRefRule {
		if !result.Photo.IsNull() {
			t.Fatal("Source ignores malformed records")
		}
		checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)
		if consumeErr := transport.AssertConsumed(); consumeErr != nil {
			t.Fatal(consumeErr)
		}
		return
	}
	photo, err := result.Photo.Get()
	if err != nil {
		t.Fatal(err)
	}
	checkSharedValueRule(t, photo, mutation)
	checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)
	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func checkSharedValueRule(t *testing.T, photo icloud.SharedPhoto, mutation string) {
	t.Helper()

	switch mutation {
	case sharedFilenameTextRule:
		if photo.Photo.Filename != "YWJj" || photo.Photo.Versions["original"].Filename != "YWJj" {
			t.Fatal("legacy filenames must decode base64 only once")
		}
	case sharedInvalidDateRule:
		if !photo.Photo.Created.Equal(time.Unix(0, 0).UTC()) {
			t.Fatal("legacy invalid dates must fall back to epoch")
		}
	case sharedFractionalSizeRule:
		if string(photo.Photo.Size) != "1" {
			t.Fatal("legacy sizes must truncate like Source int")
		}
	case sharedStringSizeRule:
		if string(photo.Photo.Size) != "0" {
			t.Fatal("Source int rejects decimal strings")
		}
	case sharedTruthyLikedRule:
		if !photo.Liked {
			t.Fatal("Source bool treats nonempty strings as true")
		}
	case sharedEmptyLikedRule:
		if photo.Liked {
			t.Fatal("Source bool treats empty arrays as false")
		}
	}
}

func sharedValueJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func mutateSharedRecordValues(mutation string, fields map[string]json.RawMessage,
	records []map[string]json.RawMessage,
) {
	switch mutation {
	case sharedFilenameTextRule:
		fields["filenameEnc"] = json.RawMessage(`{"value":"WVdKag=="}`)
	case sharedInvalidDateRule:
		fields["originalCreationDate"] = json.RawMessage(`{"value":1e300}`)
	case sharedFractionalSizeRule:
		fields["resOriginalFileSize"] = json.RawMessage(`{"value":1.75}`)
	case sharedStringSizeRule:
		fields["resOriginalFileSize"] = json.RawMessage(`{"value":"1.0"}`)
	case sharedMissingWidthRule:
		delete(fields, "resOriginalWidth")
	case sharedMissingHeightRule:
		delete(fields, "resOriginalHeight")
	case sharedTruthyLikedRule:
		records[1]["pluginFields"] = json.RawMessage(`{"likedByCaller":{"value":"false"}}`)
	case sharedEmptyLikedRule:
		records[1]["pluginFields"] = json.RawMessage(`{"likedByCaller":{"value":[]}}`)
	case sharedNullLikeCountRule:
		records[1]["pluginFields"] = json.RawMessage(`{"likeCount":{"value":null}}`)
	case sharedStringLikeCountRule:
		records[1]["pluginFields"] = json.RawMessage(`{"likeCount":{"value":"many"}}`)
	case sharedMalformedMasterRefRule:
		records[1]["fields"] = json.RawMessage(`{"masterRef":{"value":42}}`)
	}
}
