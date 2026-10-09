package replay_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const sharedProviderMutation = "provider"

func TestSharedPhotosFailureEvidence(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"albums-1", "count-1", "assets-1", "get-found", "download-binary"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join("fixtures/synthetic/http", "photos-upload-shared-"+name+".json")

			for _, bad := range []string{sharedProviderMutation, "invalid-json", reminderChangeNullValue} {
				if name == "download-binary" && bad != sharedProviderMutation {
					continue
				}

				t.Run(bad, func(t *testing.T) { runSharedPhotosFailure(t, path, bad) })
			}
		})
	}
}

func runSharedPhotosFailure(t *testing.T, path, mutation string) {
	t.Helper()
	scenario := readAccountScenario(t, path)
	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response

	body := "{"
	if mutation == reminderChangeNullValue {
		body = reminderChangeNullValue
	}

	kind := icloud.InvalidResponse

	if mutation == sharedProviderMutation {
		last.Status = 503
		body = "provider refused stream"
		kind = icloud.Unavailable
	}

	encoded, err := json.Marshal(base64.StdEncoding.EncodeToString([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}

	last.Body = replay.Entity{Encoding: testBase64Encoding, Value: encoded,
		Matchers: nil, ContentTypePattern: "", Parts: nil}

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
	auth.SharedPhotosServiceURL = "https://shared.example.invalid"
	empty, err := callSharedPhotosFailure(t, client, auth, scenario.Operation)

	checkSharedFailureEvidence(t, empty, err, kind, body, scenario)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func checkSharedFailureEvidence(t *testing.T, empty bool, err error, kind icloud.ErrorKind, body string,
	scenario accountScenario,
) {
	t.Helper()

	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response

	var failure *icloud.ClientError
	if !empty || !errors.As(err, &failure) || failure.Kind() != kind {
		t.Fatal("incorrect stream failure", err)
	}

	if string(failure.ResponseBody()) != body {
		t.Fatal("exact response body was lost")
	}

	checkSDKMetadata(t, icloud.ResponseMetadata{StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders(),
		CookieScopeURL: failure.CookieScopeURL()}, last)
	checkReminderSyncResponses(t, failure.PriorResponses(), scenario.Exchanges[:len(scenario.Exchanges)-1])
}

//nolint:wrapcheck // LIB-05: return the exact client failure for unchanged evidence assertions.
func callSharedPhotosFailure(t *testing.T, client *icloud.SDK, auth icloud.AuthContext, operation string,
) (bool, error) {
	t.Helper()

	const (
		album = "synthetic-stream-0"
		photo = "synthetic-asset-0"
	)

	switch operation {
	case "stream_albums":
		result, err := client.ListSharedPhotoAlbums(t.Context(), icloud.ListSharedPhotoAlbumsRequest{Auth: auth})

		return result == nil, err
	case "stream_count":
		result, err := client.CountSharedPhotos(t.Context(), icloud.CountSharedPhotosRequest{Auth: auth, Album: album})

		return result == nil, err
	case "stream_photos":
		result, err := client.ListSharedPhotos(t.Context(), icloud.ListSharedPhotosRequest{Auth: auth, Album: album})

		return result == nil, err
	case "stream_get":
		result, err := client.GetSharedPhoto(t.Context(), icloud.GetSharedPhotoRequest{Auth: auth, Album: album,
			PhotoID: photo})

		return result == nil, err
	case "stream_download":
		result, err := client.DownloadSharedPhoto(t.Context(), icloud.DownloadSharedPhotoRequest{
			Auth: auth, Album: album, PhotoID: photo, Version: nil})

		return result == nil, err
	default:
		t.Fatal("unregistered stream operation")

		return false, nil
	}
}
