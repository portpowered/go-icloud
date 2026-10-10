package replay_test

import (
	"encoding/json"
	"testing"
	"time"
)

const reminderCLIRevisionField = "recordChangeTag"

const reminderCLISourceRevisionField = "record_change_tag"

func checkReminderCLIReminder(t *testing.T, actual, source json.RawMessage) {
	t.Helper()

	var expected map[string]json.RawMessage

	decode(t, source, &expected)

	for before, after := range map[string]string{
		"list_id": "listID", "desc": "description", "completed_date": expectedReplayCompletedDate,
		"due_date": "dueDate", "start_date": expectedReplayStartDate, "all_day": "allDay", "time_zone": "timeZone",
		expectedSourceAlarmIDs: "alarmIDs", expectedSourceHashtagIDs: "hashtagIDs",
		expectedSourceAttachmentIDs:     "attachmentIDs",
		expectedSourceRecurrenceRuleIDs: "recurrenceRuleIDs", "parent_reminder_id": expectedParentReminderID,
		reminderCLISourceRevisionField: reminderCLIRevisionField} {
		expected[after] = expected[before]
		delete(expected, before)
	}
	// Bind Source's six fractional digits to the same public RFC3339 instant.
	for _, name := range []string{expectedReplayCompletedDate, "dueDate", expectedReplayStartDate, "created", "modified"} {
		if string(expected[name]) == reminderCLINullValue {
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
