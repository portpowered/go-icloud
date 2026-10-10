package replay_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestLegacyRemindersSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/reminders-legacy-*.json")
	if err != nil || len(paths) != 9 {
		t.Fatal("legacy reminder scenario inventory changed", err)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runLegacyRemindersSDK(t, readAccountScenario(t, path))
		})
	}
}

func runLegacyRemindersSDK(t *testing.T, scenario accountScenario) {
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
	auth.LegacyRemindersServiceURL = scenario.Initial.Origin
	result, err := client.GetLegacyRemindersSnapshot(t.Context(), icloud.GetLegacyRemindersSnapshotRequest{Auth: auth})

	response := scenario.Exchanges[0].Response

	if len(scenario.Error) != 0 {
		checkLegacyRemindersFailure(t, result, err, response)
	} else {
		if err != nil {
			t.Fatal(err)
		}

		projection, marshalErr := json.Marshal(map[string]any{
			"lists": result.Lists, replayLiteralReminders: result.Reminders,
		})
		if marshalErr != nil || !reflect.DeepEqual(accountJSON(t, projection), accountJSON(t, scenario.Result)) {
			t.Fatal("legacy reminder records lost fields or order", marshalErr)
		}

		checkSDKMetadata(t, result.Metadata, response)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func checkLegacyRemindersFailure(t *testing.T, result *icloud.GetLegacyRemindersSnapshotResult,
	err error, response *replay.Response,
) {
	t.Helper()

	kind := icloud.InvalidResponse

	switch response.Status {
	case http.StatusUnauthorized:
		kind = icloud.Unauthorized
	case http.StatusServiceUnavailable:
		kind = icloud.Unavailable
	}

	var failure *icloud.ClientError
	if result != nil || !errors.As(err, &failure) || failure.Kind() != kind || failure.StatusCode() != response.Status ||
		string(failure.ResponseBody()) != string(contractAuthBody(t, response.Body)) {
		t.Fatal("legacy reminder failure lost response evidence")
	}

	checkSDKMetadata(t, icloud.ResponseMetadata{StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders(),
		CookieScopeURL: failure.CookieScopeURL()}, response)
}
