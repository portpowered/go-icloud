package replay_test

import (
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestSharedPhotoLibraryAssetReads(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"photos-shared-library-assets-0.json", "photos-shared-library-assets-1.json",
		"photos-shared-library-assets-3.json", "photos-shared-library-assets-pagination.json",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			scenario := readAccountScenario(t, filepath.Join(replayExpectedFixturesSyntheticHTTP, name))

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

			libraries, err := client.ListPhotoLibraries(t.Context(), icloud.ListPhotoLibrariesRequest{Auth: auth})
			if err != nil {
				t.Fatal(err)
			}

			if len(libraries.Libraries) != 2 {
				t.Fatal("shared library inventory differs")
			}

			library := libraries.Libraries[1]
			if !library.Shared || !library.IsSharedLibrary {
				t.Fatal("shared scope lost")
			}

			result, err := client.ListPhotoAssets(
				t.Context(),
				icloud.ListPhotoAssetsRequest{Auth: auth, Album: photoLibraryFixtureName, Library: &library},
			)
			if err != nil {
				t.Fatal(err)
			}

			checkPhotoAssetsProjection(t, result.Photos, scenario.Result)
			checkReminderSyncResponses(t, append(libraries.Responses, result.Responses...), scenario.Exchanges)

			err = transport.AssertConsumed()
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestEmptyPhotoLibraryDiscovery(t *testing.T) {
	t.Parallel()
	scenario := readAccountScenario(t, filepath.Join(replayExpectedFixturesSyntheticHTTP, "photos-libraries-empty.json"))

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

	result, err := client.ListPhotoLibraries(t.Context(), icloud.ListPhotoLibrariesRequest{Auth: auth})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Libraries) != 1 || result.Libraries[0].ID != "root" || result.Libraries[0].Shared {
		t.Fatal("root library inventory differs")
	}

	checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}
