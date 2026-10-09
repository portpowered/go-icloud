package replay_test

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestReminderQueryCommands(t *testing.T) {
	t.Parallel()
	testReminderCLIInventory(t, "query", "reminders", 39)
}

func TestReminderListUnionCommands(t *testing.T) {
	t.Parallel()
	testReminderCLIInventory(t, "list-union", "reminder-lists", 13)
}

func testReminderCLIInventory(t *testing.T, prefix, operation string, count int) {
	t.Helper()

	paths, err := filepath.Glob("../../../../tests/replay/fixtures/synthetic/http/reminders-" + prefix + "-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != count {
		t.Fatal("CLI reminder scenario inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runReminderCommand(t, operation, readObject(t, path))
		})
	}
}

func reminderCLIQueryArgs(t *testing.T, row map[string]json.RawMessage) []string {
	t.Helper()

	var inputs []string

	decode(t, row["inputs"], &inputs)
	args := []string{"--list", inputs[0]}

	var keywords map[string]json.RawMessage

	if raw := row["keyword_inputs"]; len(raw) != 0 {
		decode(t, raw, &keywords)
	}

	if raw, exists := keywords["include_completed"]; exists {
		var completed bool

		decode(t, raw, &completed)
		args = append(args, "--include-completed="+strconv.FormatBool(completed))
	}

	if raw, exists := keywords["results_limit"]; exists {
		var limit int64

		decode(t, raw, &limit)
		args = append(args, "--page-size", strconv.FormatInt(limit, 10))
	}

	return args
}

func checkReminderCLIQueryOutcome(t *testing.T, row map[string]json.RawMessage, output []byte, err error) {
	t.Helper()

	if len(row["error"]) != 0 {
		checkReminderCommandOutcome(t, "reminders", row, output, err)

		return
	}

	if err != nil {
		t.Fatal(err)
	}

	var actual, source map[string]json.RawMessage

	decode(t, output, &actual)
	decode(t, row["result"], &source)

	if len(actual) != len(source) {
		t.Fatal("CLI query result inventory differs")
	}

	var reminders, expected []json.RawMessage

	decode(t, actual["reminders"], &reminders)
	decode(t, source["reminders"], &expected)

	if len(reminders) != len(expected) {
		t.Fatal("CLI query reminder count differs")
	}

	for index, reminder := range reminders {
		checkReminderCLIReminder(t, reminder, expected[index])
	}

	for _, name := range []string{reminderCLIAlarmsField, "triggers", reminderCLIAttachmentsField,
		"hashtags", "recurrence_rules"} {
		target := name
		if name == "recurrence_rules" {
			target = "recurrenceRules"
		}

		checkReminderCLIQueryRelated(t, actual[target], source[name])
	}
}

func checkReminderCLIQueryRelated(t *testing.T, actual, source json.RawMessage) {
	t.Helper()

	var records map[string]map[string]json.RawMessage

	decode(t, source, &records)

	for _, record := range records {
		for before, after := range map[string]string{
			"reminder_id": "reminderID", "alarm_id": "alarmID", "alarm_uid": "alarmUID",
			"trigger_id": "triggerID", "location_uid": "locationUID", "file_asset_url": "fileAssetURL",
			"file_size": "fileSize", "occurrence_count": "occurrenceCount", "first_day_of_week": "firstDayOfWeek",
			reminderCLISourceRevisionField: reminderCLIRevisionField,
		} {
			if value, exists := record[before]; exists {
				record[after] = value
				delete(record, before)
			}
		}

		reminderCLIQueryRelatedValues(t, record)
	}

	checkReminderCLIValue(t, actual, records)
}

func reminderCLIQueryRelatedValues(t *testing.T, record map[string]json.RawMessage) {
	t.Helper()

	for _, name := range []string{"latitude", "longitude", "radius"} {
		if raw, exists := record[name]; exists {
			var value float64

			decode(t, raw, &value)
			record[name] = reminderCLIEncode(t, value)
		}
	}

	if raw, exists := record["created"]; exists && string(raw) != reminderCLINullValue {
		var instant time.Time

		decode(t, raw, &instant)
		record["created"] = reminderCLIEncode(t, instant)
	}
}
