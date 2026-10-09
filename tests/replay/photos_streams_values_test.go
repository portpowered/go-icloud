package replay_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestSharedPhotosLegacyValueRules(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"filename-text", "invalid-date", "fractional-size"} {
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
	switch mutation {
	case "filename-text":
		fields["filenameEnc"] = json.RawMessage(`{"value":"WVdKag=="}`)
	case "invalid-date":
		fields["originalCreationDate"] = json.RawMessage(`{"value":1e300}`)
	case "fractional-size":
		fields["resOriginalFileSize"] = json.RawMessage(`{"value":1.75}`)
	}
	records[0]["fields"] = sharedValueJSON(t, fields)
	envelope["records"] = sharedValueJSON(t, records)
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
	auth.SharedPhotosServiceURL = "https://shared.example.invalid"
	result, err := client.GetSharedPhoto(t.Context(), icloud.GetSharedPhotoRequest{
		Auth: auth, Album: "synthetic-stream-0", PhotoID: "synthetic-asset-0"})
	if err != nil {
		t.Fatal(err)
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
	case "filename-text":
		if photo.Photo.Filename != "YWJj" || photo.Photo.Versions["original"].Filename != "YWJj" {
			t.Fatal("legacy filenames must decode base64 only once")
		}
	case "invalid-date":
		if !photo.Photo.Created.Equal(time.Unix(0, 0).UTC()) {
			t.Fatal("legacy invalid dates must fall back to epoch")
		}
	case "fractional-size":
		if string(photo.Photo.Size) != "1" {
			t.Fatal("legacy sizes must truncate like Source int")
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
