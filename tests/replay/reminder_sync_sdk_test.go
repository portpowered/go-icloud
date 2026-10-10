package replay_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestReminderSyncSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/reminders-sync-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 53 {
		t.Fatal("reminder sync scenario inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(filepath.Clean(path))
			if err != nil {
				t.Fatal(err)
			}

			var scenario accountScenario

			err = json.Unmarshal(data, &scenario)
			if err != nil {
				t.Fatal(err)
			}

			runReminderSyncSDK(t, scenario)
		})
	}
}

func runReminderSyncSDK(t *testing.T, scenario accountScenario) {
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
	auth.RemindersServiceURL = scenario.Initial.Origin

	result, err := client.GetReminderSyncCursor(t.Context(), icloud.GetReminderSyncCursorRequest{Auth: auth})
	if len(scenario.Error) != 0 {
		checkReminderSyncFailure(t, scenario, result, err)
	} else {
		if err != nil {
			t.Fatal(err)
		}

		checkSDKValue(t, result.SyncToken, scenario.Result)
		checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func checkReminderSyncResponses(t *testing.T, actual []icloud.ResponseMetadata, exchanges []replay.Exchange) {
	t.Helper()

	if len(actual) != len(exchanges) {
		t.Fatal("reminder sync response inventory differs")
	}

	for index, response := range actual {
		checkSDKMetadata(t, response, exchanges[index].Response)
	}
}

func checkReminderSyncFailure(t *testing.T, scenario accountScenario,
	result *icloud.GetReminderSyncCursorResult, err error,
) {
	t.Helper()

	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response
	kind := reminderSyncFailureKind(last.Status)

	var expected map[string]string

	decodeErr := json.Unmarshal(scenario.Error, &expected)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}

	if expected["message"] == replayExpectedChangesResponseValidationFailed {
		kind = icloud.InvalidResponse
	}

	var failure *icloud.ClientError
	if result != nil || !errors.As(err, &failure) || failure.Kind() != kind {
		t.Fatalf("sync failure classification: %v", err)
	}

	if !bytes.Equal(failure.ResponseBody(), contractAuthBody(t, last.Body)) {
		t.Fatal("sync failure body differs")
	}

	checkSDKMetadata(t, icloud.ResponseMetadata{StatusCode: failure.StatusCode(),
		Headers: failure.ResponseHeaders(), CookieScopeURL: failure.CookieScopeURL()}, last)
	checkReminderSyncResponses(t, failure.PriorResponses(), scenario.Exchanges[:len(scenario.Exchanges)-1])
}

func reminderSyncFailureKind(status int) icloud.ErrorKind {
	switch status {
	case http.StatusUnauthorized:
		return icloud.Unauthorized
	case http.StatusForbidden:
		return icloud.Forbidden
	case http.StatusTooManyRequests:
		return icloud.RateLimited
	case http.StatusServiceUnavailable:
		return icloud.Unavailable
	default:
		return icloud.Provider
	}
}
