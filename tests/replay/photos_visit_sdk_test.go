package replay_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestPhotoVisitorStopsBeforeLaterPage(t *testing.T) {
	t.Parallel()
	scenario := readAccountScenario(t,
		filepath.Join(replayExpectedFixturesSyntheticHTTP, replayLiteralPhotosAssetsPaginationJSON))
	// The recorded next page is deliberately absent: any eager follow-up fails closed.
	exchanges := scenario.Exchanges[:4]

	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	auth := sdkAccountAuth(scenario.Initial)
	auth.PhotosServiceURL = scenario.Initial.Origin
	visited := 0

	result, err := client.VisitPhotoAssets(
		t.Context(),
		icloud.ListPhotoAssetsRequest{Auth: auth, Album: photoLibraryFixtureName, Library: nil},
		func(event icloud.PhotoVisitEvent) (bool, error) {
			visited++

			if event.Photo.ID == "" {
				t.Fatal("empty visitor photo")
			}

			checkReminderSyncResponses(t, event.Responses, exchanges)

			return false, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if visited != 1 || len(result.Photos) != 1 {
		t.Fatal("visitor did not stop after first photo")
	}

	checkReminderSyncResponses(t, result.Responses, exchanges)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func TestPhotoVisitorPreservesCallerFailure(t *testing.T) {
	t.Parallel()
	scenario := readAccountScenario(t,
		filepath.Join(replayExpectedFixturesSyntheticHTTP, replayLiteralPhotosAssetsPaginationJSON))

	transport, err := replay.NewHTTPTransport(scenario.Exchanges[:4])
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	auth := sdkAccountAuth(scenario.Initial)
	auth.PhotosServiceURL = scenario.Initial.Origin
	cause := errPhotoVisitorCaller

	result, err := client.VisitPhotoAssets(
		t.Context(),
		icloud.ListPhotoAssetsRequest{Auth: auth, Album: photoLibraryFixtureName, Library: nil},
		func(_ icloud.PhotoVisitEvent) (bool, error) { return false, cause },
	)

	if result != nil || !errors.Is(err, cause) {
		t.Fatal("visitor failure lost cause or returned partial result")
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

var errPhotoVisitorCaller = errors.New("synthetic visitor failure")

func TestRecentlyAddedVisitorStopsBeforeAnotherWindow(t *testing.T) {
	t.Parallel()
	scenario := readAccountScenario(t,
		filepath.Join(replayExpectedFixturesSyntheticHTTP, "photos-recently-added-overlap-partial-next.json"))
	exchanges := scenario.Exchanges[:4]

	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	auth := sdkAccountAuth(scenario.Initial)
	auth.PhotosServiceURL = scenario.Initial.Origin
	visited := 0
	request := icloud.ListRecentlyAddedPhotosRequest{Auth: auth, Library: nil}

	result, err := client.VisitRecentlyAddedPhotos(t.Context(), request,
		func(event icloud.PhotoVisitEvent) (bool, error) {
			visited++

			checkReminderSyncResponses(t, event.Responses, exchanges)

			return false, nil
		})
	if err != nil {
		t.Fatal(err)
	}

	if visited != 1 || len(result.Photos) != 1 {
		t.Fatal("recent visitor eagerly continued")
	}

	checkReminderSyncResponses(t, result.Responses, exchanges)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}
