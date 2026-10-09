package replay_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

type reminderLookupScenario struct {
	accountScenario

	Inputs []string `json:"inputs"`
}

func TestReminderLookupSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/reminders-get-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 19 {
		t.Fatal("reminder lookup scenario inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(filepath.Clean(path))
			if err != nil {
				t.Fatal(err)
			}

			var scenario reminderLookupScenario

			err = json.Unmarshal(data, &scenario)
			if err != nil {
				t.Fatal(err)
			}

			runReminderLookupSDK(t, scenario)
		})
	}
}

func runReminderLookupSDK(t *testing.T, scenario reminderLookupScenario) {
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

	result, err := client.GetReminder(t.Context(), icloud.GetReminderRequest{Auth: auth, ReminderID: scenario.Inputs[0]})
	if len(scenario.Error) == 0 {
		if err != nil {
			t.Fatal(err)
		}

		checkReminderProjection(t, result.Reminder, scenario.Result)
		checkSDKMetadata(t, result.Metadata, scenario.Exchanges[0].Response)
	} else {
		checkReminderLookupFailure(t, scenario, result, err)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func checkReminderLookupFailure(t *testing.T, scenario reminderLookupScenario,
	result *icloud.GetReminderResult, err error,
) {
	t.Helper()

	var expected map[string]string

	decodeErr := json.Unmarshal(scenario.Error, &expected)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}

	kind := icloud.InvalidResponse
	if strings.HasPrefix(expected["message"], "Lookup reminder failed") {
		kind = icloud.Provider
	}

	if scenario.Exchanges[0].Response.Status == http.StatusServiceUnavailable {
		kind = icloud.Unavailable
	}

	if expected["type"] == "LookupError" {
		kind = icloud.NotFound
	}

	var failure *icloud.ClientError
	if result != nil || !errors.As(err, &failure) || failure.Kind() != kind {
		t.Fatalf("lookup failure classification: %v", err)
	}

	response := scenario.Exchanges[0].Response
	if string(failure.ResponseBody()) != string(contractAuthBody(t, response.Body)) {
		t.Fatal("lookup failure body differs")
	}

	checkSDKMetadata(t, icloud.ResponseMetadata{StatusCode: failure.StatusCode(),
		Headers: failure.ResponseHeaders(), CookieScopeURL: failure.CookieScopeURL()}, response)
}

func checkReminderProjection(t *testing.T, actual icloud.Reminder, expected json.RawMessage) {
	t.Helper()

	var value map[string]json.RawMessage

	err := json.Unmarshal(expected, &value)
	if err != nil {
		t.Fatal(err)
	}

	for source, target := range map[string]string{
		"list_id": "listID", "desc": "description", "completed_date": "completedDate",
		"due_date": "dueDate", "start_date": "startDate", "all_day": "allDay", "time_zone": "timeZone",
		"alarm_ids": "alarmIDs", "hashtag_ids": "hashtagIDs", "attachment_ids": "attachmentIDs",
		"recurrence_rule_ids": "recurrenceRuleIDs", "parent_reminder_id": "parentReminderID",
		"record_change_tag": "recordChangeTag"} {
		value[target] = value[source]
		delete(value, source)
	}
	// Source emits six fractional digits; compare the same instants in Go's RFC3339 form.
	for _, name := range []string{"completedDate", "dueDate", "startDate", "created", "modified"} {
		if string(value[name]) == "null" {
			continue
		}

		var text string

		err = json.Unmarshal(value[name], &text)
		if err != nil {
			t.Fatal(err)
		}

		instant, parseErr := time.Parse(time.RFC3339Nano, text)
		if parseErr != nil {
			t.Fatal(parseErr)
		}

		value[name], err = json.Marshal(instant)
		if err != nil {
			t.Fatal(err)
		}
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}

	checkSDKValue(t, actual, encoded)
}
