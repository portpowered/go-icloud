package replay_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestPhotoAssetsSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/photos-assets-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 109 {
		t.Fatal("photo asset scenario inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runPhotoAssetsSDK(t, path)
		})
	}
}

func runPhotoAssetsSDK(t *testing.T, path string) {
	t.Helper()
	scenario := readAccountScenario(t, path)

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

	actual, err := client.ListPhotoAssets(t.Context(), icloud.ListPhotoAssetsRequest{
		Auth: auth, Album: photoCountFixtureAlbum(t, path)})
	if len(scenario.Error) != 0 {
		if actual != nil {
			t.Fatal("photo enumeration returned partial results on failure")
		}

		checkPhotoReadFailure(t, scenario, nil, err)
	} else {
		if err != nil || actual == nil {
			t.Fatalf("photo asset read failed: %v", err)
		}

		checkPhotoAssetsProjection(t, actual.Photos, scenario.Result)
		checkReminderSyncResponses(t, actual.Responses, scenario.Exchanges)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func checkPhotoAssetsProjection(t *testing.T, actual []icloud.Photo, raw json.RawMessage) {
	t.Helper()

	var expected []map[string]json.RawMessage

	err := json.Unmarshal(raw, &expected)
	if err != nil {
		t.Fatal(err)
	}

	for _, photo := range expected {
		for old, name := range map[string]string{"master_id": "masterID", "item_type": "itemType",
			"is_live_photo": "isLivePhoto", "asset": "assetMetadata"} {
			photo[name] = photo[old]
			delete(photo, old)
		}

		for _, field := range []string{"created", "added"} {
			var instant time.Time

			err := json.Unmarshal(photo[field], &instant)
			if err != nil {
				t.Fatal(err)
			}

			encoded, err := json.Marshal(instant.UTC())
			if err != nil {
				t.Fatal(err)
			}

			photo[field] = encoded
		}
	}

	encoded, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}

	checkSDKValue(t, actual, encoded)
}
