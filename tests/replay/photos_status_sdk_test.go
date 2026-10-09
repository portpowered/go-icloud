package replay_test

import (
	"encoding/json"
	"errors"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
	"path/filepath"
	"testing"
)

func TestPhotosStatusSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/photos-index-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 27 {
		t.Fatal("photo initialization scenario inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) { t.Parallel(); runPhotosStatusSDK(t, readAccountScenario(t, path)) })
	}
}

func runPhotosStatusSDK(t *testing.T, scenario accountScenario) {
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

	actual, err := client.GetPhotosStatus(t.Context(), icloud.GetPhotosStatusRequest{Auth: auth})
	if len(scenario.Error) != 0 {
		checkPhotosStatusFailure(t, scenario, actual, err)
	} else {
		if err != nil {
			t.Fatal(err)
		}

		var expected map[string]json.RawMessage

		decodeErr := json.Unmarshal(scenario.Result, &expected)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}

		expected["syncToken"] = expected["sync_token"]
		delete(expected, "sync_token")

		encoded, encodeErr := json.Marshal(expected)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}

		checkSDKValue(t, struct {
			State     string `json:"state"`
			SyncToken any    `json:"syncToken"`
		}{
			State: string(actual.State), SyncToken: actual.SyncToken}, encoded)
		checkSDKMetadata(t, actual.Metadata, scenario.Exchanges[0].Response)
	}

	consumeErr := transport.AssertConsumed()
	if consumeErr != nil {
		t.Fatal(consumeErr)
	}
}

func checkPhotosStatusFailure(t *testing.T, scenario accountScenario, actual *icloud.GetPhotosStatusResult, err error) {
	t.Helper()

	var expected map[string]string

	decodeErr := json.Unmarshal(scenario.Error, &expected)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}

	response := scenario.Exchanges[0].Response

	kind := reminderSyncFailureKind(response.Status)

	if response.Status < 400 {
		kind = icloud.InvalidResponse
	}

	if expected["type"] == "PyiCloudServiceNotActivatedException" {
		kind = icloud.Unavailable
	}

	var failure *icloud.ClientError
	if actual != nil || !errors.As(err, &failure) || failure.Kind() != kind {
		t.Fatalf("photo initialization failure classification: %v", err)
	}

	if string(failure.ResponseBody()) != string(contractAuthBody(t, response.Body)) {
		t.Fatal("photo initialization failure body differs")
	}

	checkSDKMetadata(t, icloud.ResponseMetadata{StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders(),
		CookieScopeURL: failure.CookieScopeURL()}, response)

	if len(failure.PriorResponses()) != 0 {
		t.Fatal("photo initialization added prior responses")
	}
}
