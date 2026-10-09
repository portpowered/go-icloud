package replay_test

import (
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestRecentlyAddedPhotosSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/photos-recently-added-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 31 {
		t.Fatal("photo asset scenario inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runRecentlyAddedPhotosSDK(t, path)
		})
	}
}

func runRecentlyAddedPhotosSDK(t *testing.T, path string) {
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

	actual, err := client.ListRecentlyAddedPhotos(
		t.Context(),
		icloud.ListRecentlyAddedPhotosRequest{Library: nil, Auth: auth},
	)
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
