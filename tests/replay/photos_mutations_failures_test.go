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

func TestPhotoMutationInvalidAcknowledgements(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"photos-create-album", "photos-rename-album", "photos-delete-album",
		"photos-add-to-album", "photos-favorite-true", "photos-delete-asset"} {
		for _, body := range []string{"not-json", `{"records":null}`, `{"records":[{"recordName":"broken"}]}`} {
			t.Run(name+body, func(t *testing.T) { t.Parallel(); runPhotoMutationFailure(t, name, body) })
		}
	}
}
func runPhotoMutationFailure(t *testing.T, name, body string) {
	t.Helper()
	path := filepath.Join("fixtures", "synthetic", "http", name+".json")
	scenario := readAccountScenario(t, path)
	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response
	last.Body = replay.Entity{Encoding: "base64", Value: marshalFindMyRecovery(t, base64.StdEncoding.EncodeToString([]byte(body))), Matchers: nil, ContentTypePattern: "", Parts: nil}
	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}
	entropy, err := base64.StdEncoding.DecodeString("AAECAwQFBgcICQoLDA0ODw==")
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
	var operation string
	var inputs []json.RawMessage
	authReplayDecode(t, raw["operation"], &operation)
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
	checkReminderSyncResponses(t, append(prefix, failure.PriorResponses()...), scenario.Exchanges[:len(scenario.Exchanges)-1])
}

func TestPhotoAlbumCreationEmptyAcknowledgement(t *testing.T) {
	t.Parallel()
	scenario := readAccountScenario(t, "fixtures/synthetic/http/photos-create-album.json")
	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response
	last.Body = replay.Entity{Encoding: "base64", Value: marshalFindMyRecovery(t,
		base64.StdEncoding.EncodeToString([]byte(`{"records":[]}`))), Matchers: nil, ContentTypePattern: "", Parts: nil}
	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}
	entropy, err := base64.StdEncoding.DecodeString("AAECAwQFBgcICQoLDA0ODw==")
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
		Name: "New synthetic", Library: nil, Folder: false})
	if err != nil || result == nil || !result.Album.IsNull() {
		t.Fatal("empty creation ACK not explicit null", err)
	}
	checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)
	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}
