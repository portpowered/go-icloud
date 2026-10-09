package replay_test

import (
	"encoding/json"
	"testing"
)

func TestReminderRelatedCommands(t *testing.T) {
	t.Parallel()

	for prefix, operation := range map[string]string{"tags": reminderCLITagsCommand,
		reminderCLIAttachmentsField: reminderCLIAttachmentsCommand,
		"recurrence":                reminderCLIRecurrenceCommand, reminderCLIAlarmsField: reminderCLIAlarmsCommand} {
		count := map[string]int{"tags": 10, reminderCLIAttachmentsField: 11,
			"recurrence": 10, reminderCLIAlarmsField: 15}[prefix]
		testReminderCLIInventory(t, "related-"+prefix, operation, count)
	}
}
func reminderCLIRelatedArgs(t *testing.T, row map[string]json.RawMessage) []string {
	t.Helper()

	var operation string

	decode(t, row["operation"], &operation)
	key := map[string]string{"tags_for": "hashtag_ids", "attachments_for": "attachment_ids",
		"recurrence_rules_for": "recurrence_rule_ids", "alarms_for": "alarm_ids"}[operation]

	var inputs []struct {
		Value map[string]json.RawMessage `json:"value"`
	}

	decode(t, row["inputs"], &inputs)

	var ids []string

	decode(t, inputs[0].Value[key], &ids)

	args := make([]string, 0, 2*len(ids))
	for _, id := range ids {
		args = append(args, "--related-id", id)
	}

	return args
}
func checkReminderCLIRelatedOutcome(t *testing.T, operation string, row map[string]json.RawMessage,
	output []byte, err error,
) {
	t.Helper()

	if len(row["error"]) != 0 {
		checkReminderCommandOutcome(t, operation, row, output, err)

		return
	}

	if err != nil {
		t.Fatal(err)
	}

	var actual map[string]json.RawMessage

	decode(t, output, &actual)

	if len(actual) != 1 || string(actual["items"]) == reminderCLINullValue {
		t.Fatal("CLI related result contains unexpected fields or null items")
	}

	checkReminderCLIRelatedArray(t, actual["items"], row["result"])
}
func checkReminderCLIRelatedArray(t *testing.T, actual json.RawMessage, source json.RawMessage) {
	t.Helper()

	var rows []map[string]json.RawMessage

	err := json.Unmarshal(source, &rows)
	if err != nil {
		t.Fatal(err)
	}

	for _, row := range rows {
		for _, name := range []string{"alarm", "trigger"} {
			raw, exists := row[name]
			if !exists || string(raw) == reminderCLINullValue {
				continue
			}

			var nested map[string]json.RawMessage

			err = json.Unmarshal(raw, &nested)
			if err != nil {
				t.Fatal(err)
			}

			renameReminderCLIRelated(t, nested)

			encoded, err := json.Marshal(nested)
			if err != nil {
				t.Fatal(err)
			}

			row[name] = encoded
		}

		renameReminderCLIRelated(t, row)
	}

	encoded, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}

	var expected any

	decode(t, encoded, &expected)
	checkReminderCLIValue(t, actual, expected)
}

func renameReminderCLIRelated(t *testing.T, row map[string]json.RawMessage) {
	t.Helper()

	for source, target := range map[string]string{"reminder_id": "reminderID", "alarm_id": "alarmID",
		"alarm_uid": "alarmUID", "trigger_id": "triggerID",
		"location_uid": "locationUID", "file_asset_url": "fileAssetURL",
		"file_size": "fileSize", "occurrence_count": "occurrenceCount",
		"first_day_of_week":            "firstDayOfWeek",
		reminderCLISourceRevisionField: reminderCLIRevisionField} {
		if raw, exists := row[source]; exists {
			row[target] = raw
			delete(row, source)
		}
	}

	reminderCLIQueryRelatedValues(t, row)
}
