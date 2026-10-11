package replay_test

import (
	"encoding/json"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const directPhotoForbiddenVariant = "forbidden"

func TestDirectPhotoLookupDoesNotEnumerateMissingAsset(t *testing.T) {
	t.Parallel()

	scenario := readAccountScenario(t, "fixtures/sdk/photos-direct-record-missing.json")

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

	encoded, err := json.Marshal(map[string]any{"auth": auth, "album": "Library",
		"photoID": photoMutationAssetID, "directLookupOnly": true})
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

	checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)
}

func TestDirectPhotoLookupUsesBoundedRecordsWhenQueryMisses(t *testing.T) {
	t.Parallel()

	for _, variant := range []string{"found", "deleted", "hidden", directPhotoForbiddenVariant} {
		t.Run(variant, func(t *testing.T) {
			t.Parallel()

			scenario := readAccountScenario(t, "fixtures/sdk/photos-direct-record-"+variant+".json")

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
			directOnly := true

			result, err := client.GetPhoto(t.Context(), icloud.GetPhotoRequest{Auth: auth, Library: nil,
				Album: "Library", PhotoID: photoMutationAssetID, DirectLookupOnly: &directOnly})
			if variant == directPhotoForbiddenVariant {
				if err == nil || result != nil {
					t.Fatal("record permission failure was hidden as an absent photo")
				}
			} else {
				checkPhotoLookupResult(t, result, err, scenario)
			}

			err = transport.AssertConsumed()
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
