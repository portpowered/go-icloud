package replay_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const syntheticNewPhotoAlbumName = "New synthetic"

func TestPhotoMutationInvalidAcknowledgements(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		replayExpectedPhotosCreateAlbum, replayExpectedPhotosRenameAlbum, replayExpectedPhotosDeleteAlbum,
		replayExpectedPhotosAddToAlbum, replayExpectedPhotosFavoriteTrue, replayExpectedPhotosDeleteAsset} {
		for _, body := range []string{"not-json", `{"records":null}`, `{"records":[{"recordName":"broken"}]}`} {
			t.Run(name+body, func(t *testing.T) { t.Parallel(); runPhotoMutationFailure(t, name, body) })
		}
	}
}
func runPhotoMutationFailure(t *testing.T, name, body string) {
	t.Helper()

	path := filepath.Join(replayExpectedFixtures, replayExpectedSynthetic, "http", name+".json")
	scenario := readAccountScenario(t, path)
	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response
	last.Body = replay.Entity{Encoding: testBase64Encoding,
		Value:    marshalFindMyRecovery(t, base64.StdEncoding.EncodeToString([]byte(body))),
		Matchers: nil, ContentTypePattern: "", Parts: nil}

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	entropy, err := base64.StdEncoding.DecodeString(replayExpectedAlbumRecordBytes)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport), icloud.WithRandomSource(bytes.NewReader(entropy)),
		icloud.WithClock(func() time.Time { return time.Unix(1700000000, 0) }))
	if err != nil {
		t.Fatal(err)
	}

	auth := sdkAccountAuth(scenario.Initial)
	auth.PhotosServiceURL = scenario.Initial.Origin
	raw := authReplayObject(t, path)

	var (
		operation string
		inputs    []json.RawMessage
	)

	authReplayDecode(t, raw[replayExpectedOperation], &operation)
	authReplayDecode(t, raw["inputs"], &inputs)
	_, err = callPhotoMutationScenario(t, client, auth, nil, scenario, operation, name, inputs)
	checkPhotoMutationFailure(t, scenario, nil, err, icloud.InvalidResponse)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}
func checkPhotoMutationFailure(t *testing.T, scenario accountScenario, prefix []icloud.ResponseMetadata,
	callErr error, kind icloud.ErrorKind,
) {
	t.Helper()

	var failure *icloud.ClientError
	if !errors.As(callErr, &failure) || failure.Kind() != kind {
		t.Fatal("typed mutation failure mismatch", callErr)
	}

	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response
	if !bytes.Equal(failure.ResponseBody(), contractAuthBody(t, last.Body)) {
		t.Fatal("mutation failure body missing")
	}

	checkSDKMetadata(t, icloud.ResponseMetadata{StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders(),
		CookieScopeURL: failure.CookieScopeURL()}, last)
	checkReminderSyncResponses(t, append(prefix, failure.PriorResponses()...),
		scenario.Exchanges[:len(scenario.Exchanges)-1])
}

func TestPhotoAlbumCreationEmptyAcknowledgement(t *testing.T) {
	t.Parallel()
	scenario := readAccountScenario(t, replayExpectedAlbumFixturePath)
	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response
	last.Body = replay.Entity{Encoding: testBase64Encoding,
		Value: marshalFindMyRecovery(t,
			base64.StdEncoding.EncodeToString([]byte(`{"records":[]}`))),
		Matchers: nil, ContentTypePattern: "", Parts: nil}

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	entropy, err := base64.StdEncoding.DecodeString(replayExpectedAlbumRecordBytes)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport), icloud.WithRandomSource(bytes.NewReader(entropy)),
		icloud.WithClock(func() time.Time { return time.Unix(1700000000, 0) }))
	if err != nil {
		t.Fatal(err)
	}

	auth := sdkAccountAuth(scenario.Initial)
	auth.PhotosServiceURL = scenario.Initial.Origin

	result, err := client.CreatePhotoAlbum(t.Context(), icloud.CreatePhotoAlbumRequest{Auth: auth,
		Name: syntheticNewPhotoAlbumName, Library: nil, Folder: false, Type: nil})
	if err != nil || result == nil || !result.Album.IsNull() {
		t.Fatal("empty creation ACK not explicit null", err)
	}

	checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func TestPhotoFavoriteRefreshFailureKeepsFallback(t *testing.T) {
	t.Parallel()

	path := "fixtures/synthetic/http/photos-favorite-refresh.json"
	scenario := readAccountScenario(t, path)
	scenario.Exchanges[len(scenario.Exchanges)-1].Response.Status = 503

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

	result, err := client.SetPhotoFavorite(t.Context(), icloud.SetPhotoFavoriteRequest{Auth: auth,
		PhotoID: photoMutationAssetID, Favorite: true, Album: nil, Library: nil})
	if err != nil || result == nil {
		t.Fatal("failed optional refresh lost acknowledged favorite fallback", err)
	}

	var expected map[string]json.RawMessage

	authReplayDecode(t, scenario.Result, &expected)
	checkPhotoAssetsProjection(t, []icloud.Photo{result.Photo},
		append(append(json.RawMessage("["), expected["photo"]...), ']'))
	checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func TestPhotoAlbumRenameSameNameDoesNotWrite(t *testing.T) {
	t.Parallel()
	scenario := readAccountScenario(t, "fixtures/synthetic/http/photos-rename-album.json")
	scenario.Exchanges = scenario.Exchanges[:2]
	client, transport := newPhotoMutationReplay(t, scenario)
	auth := sdkAccountAuth(scenario.Initial)
	auth.PhotosServiceURL = scenario.Initial.Origin
	result, err := client.RenamePhotoAlbum(t.Context(), icloud.RenamePhotoAlbumRequest{Auth: auth,
		AlbumID: photoMutationAlbumID, Name: replayExpectedSynthetic0, Library: nil})
	if err != nil || result == nil || result.Album.GetOrEmpty().Name != replayExpectedSynthetic0 {
		t.Fatal("same-name rename failed", err)
	}
	checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)
	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

// This control derives wire expectations from the sanitized album example;
// it is not an additional captured or Python-paired scenario.
func TestPhotoAlbumCreationExplicitTypeOverridesFolder(t *testing.T) {
	t.Parallel()

	for _, kind := range []icloud.PhotoAlbumType{icloud.PhotoAlbumTypeAlbum, icloud.PhotoAlbumTypeSmartAlbum} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			t.Parallel()
			runPhotoAlbumTypeControl(t, kind)
		})
	}
}

func runPhotoAlbumTypeControl(t *testing.T, kind icloud.PhotoAlbumType) {
	t.Helper()

	scenario := readAccountScenario(t, replayExpectedAlbumFixturePath)
	last := &scenario.Exchanges[len(scenario.Exchanges)-1]
	body := contractAuthBody(t, last.Request.Body)
	oldValue := []byte(`"albumType": {"type": "INT64", "value": 0}`)
	newValue := []byte(fmt.Sprintf(`"albumType": {"type": "INT64", "value": %d}`, kind))
	if bytes.Count(body, oldValue) != 1 {
		t.Fatal("synthetic type control must replace exactly one album kind")
	}

	last.Request.Body.Value = marshalFindMyRecovery(t,
		base64.StdEncoding.EncodeToString(bytes.Replace(body, oldValue, newValue, 1)))
	client, transport := newPhotoMutationReplay(t, scenario)
	auth := sdkAccountAuth(scenario.Initial)
	auth.PhotosServiceURL = scenario.Initial.Origin
	result, err := client.CreatePhotoAlbum(t.Context(), icloud.CreatePhotoAlbumRequest{Auth: auth,
		Name: syntheticNewPhotoAlbumName, Folder: true, Library: nil, Type: &kind})
	if err != nil || result == nil || result.Album.IsNull() {
		t.Fatal("explicit album type not acknowledged", err)
	}

	checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func TestPhotoAlbumCreationEntropyFailurePreservesInitialization(t *testing.T) {
	t.Parallel()

	scenario := readAccountScenario(t, replayExpectedAlbumFixturePath)
	scenario.Exchanges = scenario.Exchanges[:1]
	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	clockCalled := false
	client, err := icloud.New(icloud.WithHTTPTransport(transport),
		icloud.WithRandomSource(bytes.NewReader(nil)), icloud.WithClock(func() time.Time {
			clockCalled = true

			return time.Unix(1700000000, 0)
		}))
	if err != nil {
		t.Fatal(err)
	}

	auth := sdkAccountAuth(scenario.Initial)
	auth.PhotosServiceURL = scenario.Initial.Origin
	_, err = client.CreatePhotoAlbum(t.Context(), icloud.CreatePhotoAlbumRequest{Auth: auth,
		Name: syntheticNewPhotoAlbumName, Folder: false, Type: nil, Library: nil})

	var failure *icloud.ClientError
	if !errors.As(err, &failure) || failure.Kind() != icloud.Configuration || !clockCalled {
		t.Fatal("entropy failure lost Source clock order or typed configuration failure", err)
	}

	checkPhotoMutationFailure(t, scenario, nil, err, icloud.Configuration)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}
