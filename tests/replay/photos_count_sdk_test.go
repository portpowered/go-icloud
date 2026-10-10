package replay_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestPhotoCountSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/photos-count-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 56 {
		t.Fatal("photo count scenario inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runPhotoCountSDK(t, path)
		})
	}
}

func runPhotoCountSDK(t *testing.T, path string) {
	t.Helper()
	scenario := readAccountScenario(t, path)

	album := photoCountFixtureAlbum(t, path)

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

	actual, err := client.GetPhotoAlbumCount(
		t.Context(),
		icloud.GetPhotoAlbumCountRequest{Library: nil, Auth: auth, Album: album},
	)
	if len(scenario.Error) != 0 {
		if actual != nil {
			t.Fatal("photo count returned partial results on failure")
		}

		checkPhotoReadFailure(t, scenario, nil, err)
	} else {
		if err != nil {
			t.Fatal(err)
		}

		checkSDKValue(t, actual.Count, scenario.Result)
		checkReminderSyncResponses(t, actual.Responses, scenario.Exchanges)
	}

	consumeErr := transport.AssertConsumed()
	if consumeErr != nil {
		t.Fatal(consumeErr)
	}
}

func photoCountFixtureAlbum(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	var row map[string]json.RawMessage

	decodeErr := json.Unmarshal(data, &row)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}

	album := photoLibraryFixtureName
	if raw, exists := row["album"]; exists {
		decodeErr := json.Unmarshal(raw, &album)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
	}

	return album
}

const photoLibraryFixtureName = "Library"
