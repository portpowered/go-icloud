package command_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestTypedPhotoReadsPairedReferenceReplay(t *testing.T) {
	t.Parallel()

	for _, scenario := range []writeReplayCase{
		{testPhotoLibrariesCommand, "photos-libraries-empty"},
		{testPhotoCursorCommand, "photos-sync-cached"},
		{testPhotoChangesCommand, "photos-changes-empty"},
		{testPhotoLibraryChangesCommand, "photos-container-private-changes-0"},
		{testPhotosRecentlyAddedCommand, "photos-recently-added-one"},
		{testSharedPhotoAlbumsCommand, "photos-upload-shared-albums-0"},
		{testSharedPhotoCountCommand, "photos-upload-shared-count-1"},
		{testSharedPhotosCommand, "photos-upload-shared-assets-1"},
		{testSharedPhotoCommand, "photos-upload-shared-get-missing"},
		{testSharedPhotoDownloadCommand, "photos-upload-shared-download-binary"},
		{testPhotoUploadStatusCommand, "photos-upload-status-1"},
	} {
		t.Run(scenario.fixture, func(t *testing.T) { t.Parallel(); runTypedReadReplay(t, scenario) })
	}
}

func runTypedReadReplay(t *testing.T, scenario writeReplayCase) {
	t.Helper()
	fixture := loadWriteFixture(t, scenario.fixture)
	transport, err := replay.NewHTTPTransport(fixture.Exchanges)
	if err != nil {
		t.Fatal(err)
	}
	client := fixtureWriteClient(t, fixture, transport)
	auth := fixtureWriteAuthentication(t, fixture)
	if origin := fixture.Initial["shared_streams_origin"]; len(origin) != 0 {
		decodeWriteFixture(t, origin, &auth.SharedPhotosServiceURL)
	}
	directory := t.TempDir()
	session := filepath.Join(directory, testSessionJsonFilename)
	request, result := filepath.Join(directory, testRequestJsonFilename), filepath.Join(directory, testResultJsonFilename)

	writeFixtureValue(t, session, auth)
	input := typedReadFixtureRequest(scenario.operation, fixture)
	input["auth"] = map[string]any{testAccountIDKey: "foreign", "photosServiceURL": "https://foreign.example.invalid"}
	writeFixtureValue(t, request, input)

	var output, diagnostic bytes.Buffer

	args := []string{testSessionFlag, session, testRequestFlag, request, testSaveResultFlag, result, scenario.operation}
	err = command.Run(t.Context(), client, args, &output, &diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
	private, err := os.ReadFile(result)
	if err != nil {
		t.Fatal(err)
	}

	var original map[string]json.RawMessage

	var actual map[string]any

	decodeWriteFixture(t, private, &original)
	decodeWriteFixture(t, output.Bytes(), &actual)

	var responses []icloud.ResponseMetadata

	decodeWriteFixture(t, original[testResponsesKey], &responses)
	checkWriteResponses(t, responses, fixture.Exchanges)
	if scenario.operation == testPhotoLibrariesCommand {
		libraries, listPresent := actual["libraries"].([]any)
		if !listPresent || len(libraries) != 1 {
			t.Fatal(testRootInventoryDiffers)
		}
		library, objectPresent := libraries[0].(map[string]any)
		if !objectPresent || library["id"] != "root" {
			t.Fatal(testRootInventoryDiffers)
		}
		return
	}
	expected := typedReadFixtureExpected(t, scenario.operation, fixture)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("console projection differs\nactual: %#v\nSource: %#v", actual, expected)
	}

	for _, secret := range []string{
		"https://assets.example.invalid", "synthetic-next", "synthetic-cached", "synthetic-account",
	} {
		if strings.Contains(output.String(), secret) {
			t.Fatal("private provider state reached ordinary output")
		}
	}
}

func typedReadFixtureRequest(operation string, fixture writeFixture) map[string]any {
	switch operation {
	case testPhotoChangesCommand:
		return map[string]any{"since": fixture.Keywords["since"]}
	case testSharedPhotoCountCommand, testSharedPhotosCommand:
		return map[string]any{"album": testSyntheticStream0}
	case testSharedPhotoCommand, testSharedPhotoDownloadCommand:
		return map[string]any{"album": testSyntheticStream0, "photoID": fixture.Inputs[0]}
	case testPhotoUploadStatusCommand:
		return map[string]any{"jobIDs": fixture.Inputs[0]}
	default:
		return map[string]any{}
	}
}

func typedReadFixtureExpected(t *testing.T, operation string, fixture writeFixture) map[string]any {
	t.Helper()

	switch operation {
	case testPhotoCursorCommand:
		return map[string]any{}
	case testPhotoChangesCommand:
		return map[string]any{"changes": []any{}}
	case testPhotoLibraryChangesCommand:
		return map[string]any{"zones": []any{}, "moreComing": false}
	case testSharedPhotoAlbumsCommand:
		return map[string]any{"albums": []any{}}
	case testSharedPhotoCountCommand:
		return map[string]any{"count": fixture.Result}
	case testSharedPhotoCommand:
		return map[string]any{"photo": nil}
	case testSharedPhotoDownloadCommand:
		return map[string]any{"content": fixture.Result}
	case testPhotosRecentlyAddedCommand, testSharedPhotosCommand:
		return map[string]any{"photos": typedReadSourcePhotos(t, fixture.Result, operation == testSharedPhotosCommand)}
	case testPhotoUploadStatusCommand:
		jobs := make(map[string]any)
		source, objectPresent := fixture.Result.(map[string]any)
		if !objectPresent {
			t.Fatal("Source upload status is not an object")
		}

		for id, raw := range source {
			job := sourceServiceObject(t, raw)
			value := sourceServiceObject(t, sourceWriteProjection(t, job["value"]))
			value["unknown"] = job["is_unknown"]
			jobs[id] = value
		}
		return map[string]any{"jobs": jobs}
	default:
		t.Fatal("missing Source read projection", operation)
		return nil
	}
}

func typedReadSourcePhotos(t *testing.T, source any, shared bool) []any {
	t.Helper()
	photos := sourceServiceList(t, sourceWriteProjection(t, source))

	for index, raw := range photos {
		photo := sourceServiceObject(t, raw)
		for _, key := range []string{testAssetMetadataKey, testMasterMetadataKey, testVersionsKey,
			testDimensionsKey, "size", testChecksumKey} {
			delete(photo, key)
		}
		if _, exists := photo[testIsLivePhotoKey]; !exists {
			photo[testIsLivePhotoKey] = false
		}
		if shared {
			likeCount, liked := photo[testLikeCountKey], photo["liked"]
			delete(photo, testLikeCountKey)
			delete(photo, "liked")
			photos[index] = map[string]any{"photo": photo, testLikeCountKey: likeCount, "liked": liked}
		}
	}
	return photos
}
