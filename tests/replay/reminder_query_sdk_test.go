package replay_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const reminderPublicRevisionField = "recordChangeTag"

const reminderSourceRevisionField = "record_change_tag"

type reminderQueryScenario struct {
	accountScenario

	Inputs []string `json:"inputs"`
	//nolint:tagliatelle // LIB-05: portable initial inputs are language independent.
	Keywords reminderQueryKeywords `json:"keyword_inputs"`
}

type reminderQueryKeywords struct {
	//nolint:tagliatelle // LIB-05: pinned Source keyword spelling.
	IncludeCompleted *bool `json:"include_completed"`
	//nolint:tagliatelle // LIB-05: pinned Source keyword spelling.
	ResultsLimit *int64 `json:"results_limit"`
}

func TestReminderQuerySDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/reminders-query-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 37 {
		t.Fatal("compound reminder query inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(filepath.Clean(path))
			if err != nil {
				t.Fatal(err)
			}

			var scenario reminderQueryScenario

			err = json.Unmarshal(data, &scenario)
			if err != nil {
				t.Fatal(err)
			}

			runReminderQuerySDK(t, scenario)
		})
	}
}

func runReminderQuerySDK(t *testing.T, scenario reminderQueryScenario) {
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

	result, err := client.ListReminders(t.Context(), icloud.ListRemindersRequest{Auth: auth,
		ListID: scenario.Inputs[0], IncludeCompleted: scenario.Keywords.IncludeCompleted,
		ResultsLimit: scenario.Keywords.ResultsLimit})
	if len(scenario.Error) != 0 {
		checkReminderQueryFailure(t, scenario, result, err)
	} else {
		if err != nil {
			t.Fatal(err)
		}

		checkReminderQueryProjection(t, result, scenario.Result)
		checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func checkReminderQueryFailure(t *testing.T, scenario reminderQueryScenario,
	result *icloud.ListRemindersResult, err error,
) {
	t.Helper()

	var failure *icloud.ClientError

	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response
	kind := reminderSyncFailureKind(last.Status)

	var expected map[string]string

	decodeErr := json.Unmarshal(scenario.Error, &expected)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}

	if expected["type"] == "ValidationError" {
		kind = icloud.InvalidResponse
	}

	if result != nil || !errors.As(err, &failure) || failure.Kind() != kind {
		t.Fatalf("compound query failure classification: %v", err)
	}

	if string(failure.ResponseBody()) != string(contractAuthBody(t, last.Body)) {
		t.Fatal("query failure body differs")
	}

	checkSDKMetadata(t, icloud.ResponseMetadata{StatusCode: failure.StatusCode(),
		Headers: failure.ResponseHeaders(), CookieScopeURL: failure.CookieScopeURL()}, last)
	checkReminderSyncResponses(t, failure.PriorResponses(), scenario.Exchanges[:len(scenario.Exchanges)-1])
}

func checkReminderQueryProjection(t *testing.T, actual *icloud.ListRemindersResult, source json.RawMessage) {
	t.Helper()

	var expected map[string]json.RawMessage

	err := json.Unmarshal(source, &expected)
	if err != nil {
		t.Fatal(err)
	}

	var reminders []json.RawMessage

	err = json.Unmarshal(expected["reminders"], &reminders)
	if err != nil {
		t.Fatal(err)
	}

	if actual.Reminders == nil || len(actual.Reminders) != len(reminders) {
		t.Fatal("scoped reminder inventory differs")
	}

	for index, reminder := range actual.Reminders {
		checkReminderProjection(t, reminder, reminders[index])
	}

	for name, value := range map[string]any{"alarms": actual.Alarms, "triggers": actual.Triggers,
		"attachments": actual.Attachments, "hashtags": actual.Hashtags, "recurrence_rules": actual.RecurrenceRules} {
		checkReminderRelatedProjection(t, value, expected[name])
	}
}

func checkReminderRelatedProjection(t *testing.T, actual any, source json.RawMessage) {
	t.Helper()

	var records map[string]map[string]json.RawMessage

	err := json.Unmarshal(source, &records)
	if err != nil {
		t.Fatal(err)
	}

	for _, record := range records {
		for source, target := range map[string]string{"reminder_id": "reminderID", "alarm_id": "alarmID",
			"alarm_uid": "alarmUID", "trigger_id": "triggerID", "location_uid": "locationUID",
			"file_asset_url": "fileAssetURL", "file_size": "fileSize", "occurrence_count": "occurrenceCount",
			"first_day_of_week": "firstDayOfWeek", reminderSourceRevisionField: reminderPublicRevisionField} {
			if raw, exists := record[source]; exists {
				record[target] = raw
				delete(record, source)
			}
		}

		normalizeReminderRelatedValues(t, record)
	}

	encoded, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}

	checkSDKValue(t, actual, encoded)
}

func normalizeReminderRelatedValues(t *testing.T, record map[string]json.RawMessage) {
	t.Helper()

	for _, name := range []string{"latitude", "longitude", "radius"} {
		if raw, exists := record[name]; exists {
			var value float64

			err := json.Unmarshal(raw, &value)
			if err != nil {
				t.Fatal(err)
			}

			record[name], err = json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	if raw, exists := record["created"]; exists && string(raw) != reminderChangeNullValue {
		var instant time.Time

		err := json.Unmarshal(raw, &instant)
		if err != nil {
			t.Fatal(err)
		}

		record["created"], err = json.Marshal(instant)
		if err != nil {
			t.Fatal(err)
		}
	}
}
