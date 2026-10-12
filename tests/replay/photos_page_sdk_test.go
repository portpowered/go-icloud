package replay_test

import (
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestPhotoPageReturnsContinuationWithoutReadingLaterPages(t *testing.T) {
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
	limit := 100

	result, err := client.ListPhotoAssets(t.Context(), icloud.ListPhotoAssetsRequest{
		Auth: auth, Album: photoLibraryFixtureName, MaxResults: &limit, Offset: nil, Library: nil})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Photos) != 50 || result.NextOffset == nil || *result.NextOffset != 0 {
		t.Fatalf("page lost photos or rank: %+v", result)
	}
	checkReminderSyncResponses(t, result.Responses, scenario.Exchanges[:4])

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func TestPhotoPageSmallLimitAdvancesOnlyReturnedAssetRanks(t *testing.T) {
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
	limit := 1
	result, err := client.ListPhotoAssets(t.Context(), icloud.ListPhotoAssetsRequest{
		Auth: auth, Album: photoLibraryFixtureName, MaxResults: &limit, Offset: nil, Library: nil})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Photos) != 1 || result.NextOffset == nil || *result.NextOffset != 49 {
		t.Fatal("small page skipped unreturned provider assets")
	}
	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func TestPhotoPageResumesAtRankWithoutRecountingOrReadingPriorPage(t *testing.T) {
	t.Parallel()
	scenario := readAccountScenario(t,
		filepath.Join(replayExpectedFixturesSyntheticHTTP, replayLiteralPhotosAssetsPaginationJSON))
	exchanges := scenario.Exchanges[:2:2]
	exchanges = append(exchanges, scenario.Exchanges[4])

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
	limit := 100
	offset := int64(0)

	result, err := client.ListPhotoAssets(t.Context(), icloud.ListPhotoAssetsRequest{
		Auth: auth, Album: photoLibraryFixtureName, MaxResults: &limit, Offset: &offset, Library: nil})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Photos) != 1 || result.NextOffset != nil {
		t.Fatal("final page was skipped or did not terminate")
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}
