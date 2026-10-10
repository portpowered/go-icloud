package replay_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestPhotoMutationSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"photos-create-album",
		"photos-create-folder",
		"photos-rename-album",
		"photos-delete-album",
		"photos-add-to-album",
		"photos-favorite-true",
		"photos-favorite-false",
		"photos-favorite-refresh",
		"photos-favorite-record-error",
		"photos-delete-asset",
		"photos-delete-asset-record-rejection",
		"photos-shared-library-favorite-true",
		"photos-shared-library-favorite-record-error",
		"photos-shared-library-create-album",
		"photos-shared-library-create-album-refused-4"} {
		t.Run(name, func(t *testing.T) { t.Parallel(); runPhotoMutationScenario(t, name) })
	}
}
func runPhotoMutationScenario(t *testing.T, name string) {
	t.Helper()

	path := filepath.Join("fixtures", "synthetic", "http", name+".json")
	scenario := readAccountScenario(t, path)

	raw := authReplayObject(t, path)

	var operation string
	authReplayDecode(t, raw["operation"], &operation)

	var inputs []json.RawMessage
	authReplayDecode(t, raw["inputs"], &inputs)

	client, transport := newPhotoMutationReplay(t, scenario)

	var err error

	auth := sdkAccountAuth(scenario.Initial)
	auth.PhotosServiceURL = scenario.Initial.Origin

	var (
		metadata []icloud.ResponseMetadata
		prefix   []icloud.ResponseMetadata
		library  *icloud.PhotoLibrary
	)

	library, prefix = discoverPhotoMutationLibrary(t, client, auth, name)

	metadata, err = callPhotoMutationScenario(t, client, auth, library, scenario, operation, name, inputs)

	if len(scenario.Error) != 0 || name == "photos-delete-asset-record-rejection" {
		kind := icloud.Provider
		if scenario.Exchanges[len(scenario.Exchanges)-1].Response.Status >= 400 {
			kind = reminderSyncFailureKind(scenario.Exchanges[len(scenario.Exchanges)-1].Response.Status)
		}
		checkPhotoMutationFailure(t, scenario, prefix, err, kind)
	} else {
		if err != nil {
			t.Fatal(err, errors.Unwrap(err), transport.AssertConsumed())
		}

		checkReminderSyncResponses(t, append(prefix, metadata...), scenario.Exchanges)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}
func checkPhotoMutationAlbum(t *testing.T, album icloud.PhotoAlbum, raw json.RawMessage) {
	t.Helper()

	var expected map[string]json.RawMessage
	authReplayDecode(t, raw, &expected)
	expected["fullName"] = expected["fullname"]
	delete(expected, "fullname")
	expected["recordChangeTag"] = expected["record_change_tag"]
	delete(expected, "record_change_tag")

	encoded, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}

	checkSDKValue(t, album, encoded)
}

func callPhotoMutationScenario(t *testing.T, client *icloud.SDK, auth icloud.AuthContext, library *icloud.PhotoLibrary,
	scenario accountScenario, operation, name string, inputs []json.RawMessage,
) ([]icloud.ResponseMetadata, error) {
	t.Helper()

	switch operation {
	case "create_album", "album_rename", "album_delete":
		return callPhotoAlbumMutation(t, client, auth, library, scenario, operation, name, inputs)
	default:
		return callPhotoAssetMutation(t, client, auth, library, scenario, operation, inputs)
	}
}

const (
	photoMutationAlbumID = "synthetic-album-0"
	photoMutationAssetID = "synthetic-asset-0"
)

func discoverPhotoMutationLibrary(t *testing.T, client *icloud.SDK, auth icloud.AuthContext, name string,
) (*icloud.PhotoLibrary, []icloud.ResponseMetadata) {
	t.Helper()

	var (
		library *icloud.PhotoLibrary
		prefix  []icloud.ResponseMetadata
	)

	if strings.HasPrefix(name, "photos-shared-library-") {
		discovered, callErr := client.ListPhotoLibraries(t.Context(), icloud.ListPhotoLibrariesRequest{Auth: auth})
		if callErr != nil {
			t.Fatal(callErr)
		}

		prefix = discovered.Responses
		for _, candidate := range discovered.Libraries {
			if candidate.IsSharedLibrary {
				selected := candidate
				library = &selected

				break
			}
		}

		if library == nil {
			t.Fatal("shared library missing")
		}
	}

	return library, prefix
}

func callPhotoAlbumMutation(t *testing.T, client *icloud.SDK, auth icloud.AuthContext, library *icloud.PhotoLibrary,
	scenario accountScenario, operation, name string, inputs []json.RawMessage,
) ([]icloud.ResponseMetadata, error) {
	t.Helper()

	var (
		metadata []icloud.ResponseMetadata
		err      error
	)

	switch operation {
	case "create_album":
		var value string
		authReplayDecode(t, inputs[0], &value)
		result,
			callErr := client.CreatePhotoAlbum(t.Context(),
			icloud.CreatePhotoAlbumRequest{Auth: auth,
				Name:    value,
				Folder:  name == "photos-create-folder",
				Type:    nil,
				Library: library})
		err = callErr

		if result != nil {
			metadata = result.Responses
			checkPhotoMutationAlbum(t, result.Album.GetOrEmpty(), scenario.Result)
		}
	case "album_rename":
		var value string
		authReplayDecode(t, inputs[0], &value)
		result,
			callErr := client.RenamePhotoAlbum(t.Context(),
			icloud.RenamePhotoAlbumRequest{Auth: auth,
				AlbumID: photoMutationAlbumID,
				Name:    value,
				Library: library})
		err = callErr

		if result != nil {
			metadata = result.Responses

			var expected map[string]json.RawMessage
			authReplayDecode(t, scenario.Result, &expected)
			checkPhotoMutationAlbum(t, result.Album.GetOrEmpty(), expected["album"])
		}
	case "album_delete":
		result,
			callErr := client.DeletePhotoAlbum(t.Context(),
			icloud.DeletePhotoAlbumRequest{Auth: auth,
				AlbumID: photoMutationAlbumID,
				Library: library})
		err = callErr

		if result != nil {
			metadata = result.Responses
			requirePhotoMutationAcknowledged(t, result.Deleted)
		}
	}

	return metadata, err
}

func callPhotoAssetMutation(t *testing.T, client *icloud.SDK, auth icloud.AuthContext, library *icloud.PhotoLibrary,
	scenario accountScenario, operation string, inputs []json.RawMessage,
) ([]icloud.ResponseMetadata, error) {
	t.Helper()

	var (
		metadata []icloud.ResponseMetadata
		err      error
	)

	switch operation {
	case "album_add_photo":
		result,
			callErr := client.AddPhotoToAlbum(t.Context(),
			icloud.AddPhotoToAlbumRequest{Auth: auth,
				AlbumID: photoMutationAlbumID,
				PhotoID: photoMutationAssetID,
				Library: nil})
		err = callErr

		if result != nil {
			metadata = result.Responses
			requirePhotoMutationAcknowledged(t, result.Added)
		}
	case "photo_favorite":
		var value bool
		authReplayDecode(t, inputs[1], &value)
		result,
			callErr := client.SetPhotoFavorite(t.Context(),
			icloud.SetPhotoFavoriteRequest{Auth: auth,
				PhotoID:  photoMutationAssetID,
				Favorite: value,
				Album:    nil,
				Library:  library})
		err = callErr

		if result != nil {
			metadata = result.Responses

			checkPhotoMutationPhoto(t, result.Photo, scenario.Result)
		}
	case "photo_delete":
		result,
			callErr := client.DeletePhoto(t.Context(),
			icloud.DeletePhotoRequest{Auth: auth,
				PhotoID: photoMutationAssetID,
				Album:   nil,
				Library: nil})
		err = callErr

		if result != nil {
			metadata = result.Responses
			requirePhotoMutationAcknowledged(t, result.Deleted)
		}
	default:
		t.Fatal("unknown mutation", operation)
	}

	return metadata, err
}

func newPhotoMutationReplay(t *testing.T, scenario accountScenario) (*icloud.SDK, *replay.HTTPTransport) {
	t.Helper()

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	entropy, err := base64.StdEncoding.DecodeString("AAECAwQFBgcICQoLDA0ODw==")
	if err != nil {
		t.Fatal(err)
	}

	client,
		err := icloud.New(icloud.WithHTTPTransport(transport),
		icloud.WithRandomSource(bytes.NewReader(entropy)),
		icloud.WithClock(func() time.Time {
			return time.Unix(1700000000,
				0)
		}))
	if err != nil {
		t.Fatal(err)
	}

	return client, transport
}

func requirePhotoMutationAcknowledged(t *testing.T, acknowledged bool) {
	t.Helper()
	if !acknowledged {
		t.Fatal("mutation not acknowledged")
	}
}

func checkPhotoMutationPhoto(t *testing.T, photo icloud.Photo, raw json.RawMessage) {
	t.Helper()
	var expected map[string]json.RawMessage
	authReplayDecode(t, raw, &expected)
	checkPhotoAssetsProjection(t, []icloud.Photo{photo},
		append(append(json.RawMessage("["), expected["photo"]...), ']'))
}
