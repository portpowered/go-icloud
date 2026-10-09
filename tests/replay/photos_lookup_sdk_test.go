package replay_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestPhotoLookupSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/photos-get-*.json")
	if err != nil || len(paths) != 22 {
		t.Fatal("photo lookup inventory changed", err)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runPhotoLookupSDK(t, path)
		})
	}
}

func runPhotoLookupSDK(t *testing.T, path string) {
	t.Helper()
	scenario := readAccountScenario(t, path)
	raw := authReplayObject(t, path)

	var inputs []string

	authReplayDecode(t, raw["inputs"], &inputs)

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

	result, err := client.GetPhoto(t.Context(), icloud.GetPhotoRequest{Library: nil, Auth: auth,
		Album: photoCountFixtureAlbum(t, path), PhotoID: inputs[0]})
	if len(scenario.Error) != 0 {
		if result != nil {
			t.Fatal("photo lookup returned partial output")
		}

		checkPhotoReadFailure(t, scenario, nil, err)
	} else {
		checkPhotoLookupResult(t, result, err, scenario)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func checkPhotoLookupResult(t *testing.T, result *icloud.GetPhotoResult, err error, scenario accountScenario) {
	t.Helper()

	if err != nil || result == nil {
		t.Fatal("photo lookup failed", err)
	}

	if string(scenario.Result) == reminderChangeNullValue {
		if !result.Photo.IsNull() {
			t.Fatal("absent photo was not explicit null")
		}
	} else {
		photo, photoErr := result.Photo.Get()
		if photoErr != nil {
			t.Fatal(photoErr)
		}

		expected := append(append(json.RawMessage("["), scenario.Result...), ']')
		checkPhotoAssetsProjection(t, []icloud.Photo{photo}, expected)
	}

	checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)
}
