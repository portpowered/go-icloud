package replay_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestSharedPhotosSDKScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/photos-upload-shared-*.json")
	if err != nil || len(paths) != 14 {
		t.Fatal("shared stream inventory changed", len(paths), err)
	}

	paths = append(paths, "fixtures/synthetic/http/photos-shared-streams-unavailable.json")
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) { t.Parallel(); runSharedPhotosSDK(t, path) })
	}
}

func runSharedPhotosSDK(t *testing.T, path string) {
	t.Helper()
	scenario := readAccountScenario(t, path)
	row := authReplayObject(t, path)
	initial := map[string]json.RawMessage{}
	authReplayDecode(t, row["initial_state"], &initial)

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err, errors.Unwrap(err))
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err, errors.Unwrap(err))
	}

	auth := sdkAccountAuth(scenario.Initial)

	auth.PhotosServiceURL = scenario.Initial.Origin
	if len(initial["shared_streams_origin"]) != 0 && string(initial["shared_streams_origin"]) != reminderChangeNullValue {
		authReplayDecode(t, initial["shared_streams_origin"], &auth.SharedPhotosServiceURL)
	}

	var (
		album  string
		inputs []string
	)

	if len(row["album"]) != 0 {
		authReplayDecode(t, row["album"], &album)
	}

	authReplayDecode(t, row["inputs"], &inputs)
	checkSharedPhotosOperation(t, client, auth, album, inputs, scenario)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err, errors.Unwrap(err))
	}
}

func checkSharedPhotosOperation(t *testing.T, client *icloud.SDK, auth icloud.AuthContext,
	album string, inputs []string, scenario accountScenario,
) {
	t.Helper()

	switch scenario.Operation {
	case "stream_albums", "shared_streams":
		checkSharedAlbumsCall(t, client, auth, scenario)
	case "stream_count":
		result, err := client.CountSharedPhotos(t.Context(), icloud.CountSharedPhotosRequest{Auth: auth, Album: album})
		if err != nil {
			t.Fatal(err)
		}

		checkSDKValue(t, result.Count, scenario.Result)
		checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)
	case "stream_photos":
		result, err := client.ListSharedPhotos(t.Context(), icloud.ListSharedPhotosRequest{Auth: auth, Album: album})
		if err != nil {
			t.Fatal(err)
		}

		checkSharedPhotosProjection(t, result.Photos, scenario.Result)
		checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)
	case "stream_get":
		checkSharedPhotoCall(t, client, auth, album, inputs[0], scenario)
	case "stream_download":
		result, err := client.DownloadSharedPhoto(t.Context(), icloud.DownloadSharedPhotoRequest{
			Auth: auth, Album: album, PhotoID: inputs[0], Version: nil})
		if err != nil {
			t.Fatal(err)
		}

		checkPhotoDownloadResult(t, scenario,
			&icloud.DownloadPhotoResult{Content: result.Content, Responses: result.Responses}, nil)
	default:
		t.Fatal("unregistered shared operation", scenario.Operation)
	}
}

func checkSharedAlbumsCall(t *testing.T, client *icloud.SDK, auth icloud.AuthContext, scenario accountScenario) {
	t.Helper()
	result, err := client.ListSharedPhotoAlbums(t.Context(), icloud.ListSharedPhotoAlbumsRequest{Auth: auth})

	if len(scenario.Error) != 0 {
		var failure *icloud.ClientError
		if result != nil || !errors.As(err, &failure) || failure.Kind() != icloud.Unavailable {
			t.Fatal(err)
		}

		last := scenario.Exchanges[len(scenario.Exchanges)-1].Response
		if string(failure.ResponseBody()) != string(contractAuthBody(t, last.Body)) {
			t.Fatal("unavailable body differs")
		}

		checkSDKMetadata(t, icloud.ResponseMetadata{StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders(),
			CookieScopeURL: failure.CookieScopeURL()}, last)
		checkReminderSyncResponses(t, failure.PriorResponses(), scenario.Exchanges[:len(scenario.Exchanges)-1])

		return
	}

	if err != nil {
		t.Fatal(err)
	}

	checkSharedAlbumProjection(t, result.Albums, scenario.Result)
	checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)
}

func checkSharedPhotoCall(t *testing.T, client *icloud.SDK, auth icloud.AuthContext, album, photoID string,
	scenario accountScenario,
) {
	t.Helper()

	result, err := client.GetSharedPhoto(t.Context(), icloud.GetSharedPhotoRequest{
		Auth: auth, Album: album, PhotoID: photoID})
	if err != nil {
		t.Fatal(err)
	}

	if string(scenario.Result) == reminderChangeNullValue {
		if !result.Photo.IsNull() {
			t.Fatal("missing photo must remain null")
		}
	} else {
		photo, err := result.Photo.Get()
		if err != nil {
			t.Fatal(err)
		}

		checkSharedPhotosProjection(t, []icloud.SharedPhoto{photo}, append(append([]byte("["), scenario.Result...), ']'))
	}

	checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)
}

func checkSharedAlbumProjection(t *testing.T, actual []icloud.SharedPhotoAlbum, raw json.RawMessage) {
	t.Helper()

	expected := []map[string]json.RawMessage{}
	authReplayDecode(t, raw, &expected)

	if len(actual) != len(expected) {
		t.Fatal("album count differs")
	}

	for index, album := range actual {
		checkSharedFields(t, album, expected[index], map[string]string{"fullname": "fullName", "creation_date": "created",
			"sharing_type": "sharingType", "allow_contributions": "allowContributions", "is_public": "isPublic",
			"is_web_upload_supported": "isWebUploadSupported", "public_url": "publicURL"})
	}
}

func checkSharedPhotosProjection(t *testing.T, actual []icloud.SharedPhoto, raw json.RawMessage) {
	t.Helper()

	expected := []map[string]json.RawMessage{}
	authReplayDecode(t, raw, &expected)

	if len(actual) != len(expected) {
		t.Fatal("photo count differs")
	}

	for index, photo := range actual {
		checkSDKValue(t, photo.LikeCount, expected[index]["like_count"])
		checkSDKValue(t, photo.Liked, expected[index]["liked"])
		delete(expected[index], "like_count")
		delete(expected[index], "liked")
		checkSharedFields(t, photo.Photo, expected[index], map[string]string{
			"master_id": "masterID", "item_type": "itemType"})
	}
}

func checkSharedFields(t *testing.T, actual any, expected map[string]json.RawMessage, rename map[string]string) {
	t.Helper()

	encoded, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err, errors.Unwrap(err))
	}

	fields := map[string]json.RawMessage{}
	authReplayDecode(t, encoded, &fields)

	for source, value := range expected {
		name := source
		if replacement, found := rename[source]; found {
			name = replacement
		}

		if name == "created" || name == "added" {
			var instant time.Time

			authReplayDecode(t, value, &instant)

			value, err = json.Marshal(instant.UTC())
			if err != nil {
				t.Fatal(err, errors.Unwrap(err))
			}
		}

		if name == "versions" {
			checkSharedVersions(t, fields[name], value)

			continue
		}

		checkSDKValue(t, fields[name], value)
	}
}

func checkSharedVersions(t *testing.T, actual, expected json.RawMessage) {
	t.Helper()

	actualVersions := map[string]map[string]json.RawMessage{}
	expectedVersions := map[string]map[string]json.RawMessage{}

	authReplayDecode(t, actual, &actualVersions)
	authReplayDecode(t, expected, &expectedVersions)

	if len(actualVersions) != len(expectedVersions) {
		t.Fatal("shared rendition inventory differs")
	}

	for name, version := range expectedVersions {
		for key, value := range version {
			checkSDKValue(t, actualVersions[name][key], value)
		}
	}
}
