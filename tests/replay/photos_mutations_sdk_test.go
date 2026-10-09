package replay_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
	"path/filepath"
	"testing"
	"time"
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
		"photos-shared-library-favorite-record-error"} {
		t.Run(name, func(t *testing.T) { t.Parallel(); runPhotoMutationScenario(t, name) })
	}
}
func runPhotoMutationScenario(t *testing.T, name string) {
	t.Helper()

	path := filepath.Join("fixtures", "synthetic", "http", name+".json")
	scenario := readAccountScenario(t, path)

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	raw := authReplayObject(t, path)

	var operation string
	authReplayDecode(t, raw["operation"], &operation)

	var inputs []json.RawMessage
	authReplayDecode(t, raw["inputs"], &inputs)

	entropy := []byte{}
	if operation == "create_album" {
		entropy, _ = base64.StdEncoding.DecodeString("AAECAwQFBgcICQoLDA0ODw==")
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
		checkPhotoMutationFailure(t, scenario, prefix, err, icloud.Provider)
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
	var metadata []icloud.ResponseMetadata
	var err error

	switch operation {
	case "create_album":
		var value string
		authReplayDecode(t, inputs[0], &value)
		result,
			callErr := client.CreatePhotoAlbum(t.Context(),
			icloud.CreatePhotoAlbumRequest{Auth: auth,
				Name:    value,
				Folder:  name == "photos-create-folder",
				Library: nil})
		err = callErr

		if result != nil {
			metadata = result.Responses
			checkPhotoMutationAlbum(t, result.Album, scenario.Result)
		}
	case "album_rename":
		var value string
		authReplayDecode(t, inputs[0], &value)
		result,
			callErr := client.RenamePhotoAlbum(t.Context(),
			icloud.RenamePhotoAlbumRequest{Auth: auth,
				AlbumID: photoMutationAlbumID,
				Name:    value,
				Library: nil})
		err = callErr

		if result != nil {
			metadata = result.Responses

			var expected map[string]json.RawMessage
			authReplayDecode(t, scenario.Result, &expected)
			checkPhotoMutationAlbum(t, result.Album, expected["album"])
		}
	case "album_delete":
		result,
			callErr := client.DeletePhotoAlbum(t.Context(),
			icloud.DeletePhotoAlbumRequest{Auth: auth,
				AlbumID: photoMutationAlbumID,
				Library: nil})
		err = callErr

		if result != nil {
			metadata = result.Responses
			if !result.Deleted {
				t.Fatal("album not deleted")
			}
		}
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
			if !result.Added {
				t.Fatal("relation not added")
			}
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

			var expected map[string]json.RawMessage
			authReplayDecode(t, scenario.Result, &expected)
			checkPhotoAssetsProjection(t,
				[]icloud.Photo{result.Photo},
				append(append(json.RawMessage("["),
					expected["photo"]...),
					']'))
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
			if !result.Deleted {
				t.Fatal("asset not deleted")
			}
		}
	default:
		t.Fatal("unknown mutation", operation)
	}

	return metadata, err
}

const (
	photoMutationAlbumID = "synthetic-album-0"
	photoMutationAssetID = "synthetic-asset-0"
)

func discoverPhotoMutationLibrary(t *testing.T, client *icloud.SDK, auth icloud.AuthContext, name string,
) (*icloud.PhotoLibrary, []icloud.ResponseMetadata) {
	t.Helper()
	var library *icloud.PhotoLibrary
	var prefix []icloud.ResponseMetadata

	if name == "photos-shared-library-favorite-true" || name == "photos-shared-library-favorite-record-error" {
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
