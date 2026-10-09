package replay_test

import (
	"encoding/json"
	"testing"
	"time"
)

func checkReminderCLIReminder(t *testing.T, actual, source json.RawMessage) {
	t.Helper()

	var expected map[string]json.RawMessage

	decode(t, source, &expected)

	for before, after := range map[string]string{
		"list_id": "listID", "desc": "description", "completed_date": "completedDate",
		"due_date": "dueDate", "start_date": "startDate", "all_day": "allDay", "time_zone": "timeZone",
		"alarm_ids": "alarmIDs", "hashtag_ids": "hashtagIDs", "attachment_ids": "attachmentIDs",
		"recurrence_rule_ids": "recurrenceRuleIDs", "parent_reminder_id": "parentReminderID",
		"record_change_tag": "recordChangeTag"} {
		expected[after] = expected[before]
		delete(expected, before)
	}
	// Bind Source's six fractional digits to the same public RFC3339 instant.
	for _, name := range []string{"completedDate", "dueDate", "startDate", "created", "modified"} {
		if string(expected[name]) == "null" {
			continue
		}

		var text string

		decode(t, expected[name], &text)

		instant, err := time.Parse(time.RFC3339Nano, text)
		if err != nil {
			t.Fatal(err)
		}

		expected[name] = reminderCLIEncode(t, instant)
	}

	checkReminderCLIValue(t, actual, expected)
}
