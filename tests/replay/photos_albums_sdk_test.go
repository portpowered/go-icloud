package replay_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestPhotoAlbumsSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/photos-albums-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 41 {
		t.Fatal("photo album scenario inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runPhotoAlbumsSDK(t, readAccountScenario(t, path))
		})
	}
}

func runPhotoAlbumsSDK(t *testing.T, scenario accountScenario) {
	t.Helper()

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

	actual, err := client.ListPhotoAlbums(t.Context(), icloud.ListPhotoAlbumsRequest{Auth: auth})
	if len(scenario.Error) != 0 {
		checkPhotoReadFailure(t, scenario, actual, err)
	} else {
		if err != nil {
			t.Fatal(err)
		}

		var expected []map[string]json.RawMessage

		decodeErr := json.Unmarshal(scenario.Result, &expected)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}

		for _, album := range expected {
			album["fullName"] = album["fullname"]
			delete(album, "fullname")
			album[reminderPublicRevisionField] = album[reminderSourceRevisionField]
			delete(album, reminderSourceRevisionField)
		}

		encoded, encodeErr := json.Marshal(expected)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}

		checkSDKValue(t, actual.Albums, encoded)
		checkReminderSyncResponses(t, actual.Responses, scenario.Exchanges)
	}

	consumeErr := transport.AssertConsumed()
	if consumeErr != nil {
		t.Fatal(consumeErr)
	}
}

func checkPhotoReadFailure(t *testing.T, scenario accountScenario, actual *icloud.ListPhotoAlbumsResult, err error) {
	t.Helper()

	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response

	var expected map[string]string

	decodeErr := json.Unmarshal(scenario.Error, &expected)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}

	kind := reminderSyncFailureKind(last.Status)

	if last.Status < 400 {
		kind = icloud.InvalidResponse
	}

	if expected["type"] == "KeyError" {
		kind = icloud.NotFound
	}

	if expected["type"] == "PyiCloudServiceNotActivatedException" {
		kind = icloud.Unavailable
	}

	var failure *icloud.ClientError
	if actual != nil || !errors.As(err, &failure) || failure.Kind() != kind {
		t.Fatalf("photo album failure classification: %v", err)
	}

	if string(failure.ResponseBody()) != string(contractAuthBody(t, last.Body)) {
		t.Fatal("photo album failure body differs")
	}

	checkSDKMetadata(t, icloud.ResponseMetadata{StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders(),
		CookieScopeURL: failure.CookieScopeURL()}, last)
	checkReminderSyncResponses(t, failure.PriorResponses(), scenario.Exchanges[:len(scenario.Exchanges)-1])
}
