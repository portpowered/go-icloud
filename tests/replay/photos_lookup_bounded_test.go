package replay_test

import (
	"encoding/json"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestDirectPhotoLookupDoesNotEnumerateMissingAsset(t *testing.T) {
	t.Parallel()

	scenario := readAccountScenario(t, "fixtures/synthetic/http/photos-get-missing.json")

	transport, err := replay.NewHTTPTransport(scenario.Exchanges[:3])
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	auth := sdkAccountAuth(scenario.Initial)
	auth.PhotosServiceURL = scenario.Initial.Origin

	encoded, err := json.Marshal(map[string]any{"auth": auth, "album": "Library",
		"photoID": "synthetic-missing", "directLookupOnly": true})
	if err != nil {
		t.Fatal(err)
	}

	var request icloud.GetPhotoRequest

	err = json.Unmarshal(encoded, &request)
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.GetPhoto(t.Context(), request)
	if err != nil || result == nil || !result.Photo.IsNull() {
		t.Fatal("missing direct lookup should return null without enumerating the album", err)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}

	checkReminderSyncResponses(t, result.Responses, scenario.Exchanges[:3])
}
