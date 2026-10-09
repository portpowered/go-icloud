package replay_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

const reminderChangesCommand = "reminder-changes"

const reminderCLINullValue = "null"

func reminderCLIChangesArgs(t *testing.T, row map[string]json.RawMessage) []string {
	t.Helper()

	var keywords map[string]json.RawMessage

	if len(row["keyword_inputs"]) == 0 {
		return nil
	}

	decode(t, row["keyword_inputs"], &keywords)

	raw, supplied := keywords["since"]
	if !supplied || string(raw) == reminderCLINullValue {
		return nil
	}

	var since string

	decode(t, raw, &since)

	return []string{"--since", since}
}

func checkReminderCLIChangesOutcome(t *testing.T, row map[string]json.RawMessage, output []byte, err error) {
	t.Helper()

	if len(row["error"]) != 0 {
		var failure *icloud.ClientError
		if len(output) != 0 || !errors.As(err, &failure) {
			t.Fatal("CLI lost change error or printed partial data")
		}

		checkReminderCLIFailure(t, row, failure)
		checkReminderCLIPrior(t, row, failure)

		return
	}

	if err != nil {
		t.Fatal(err)
	}

	var actual map[string]json.RawMessage

	decode(t, output, &actual)

	if len(actual) != 1 || string(actual["changes"]) == reminderCLINullValue {
		t.Fatal("CLI changes result contains unexpected fields or a null event array")
	}

	checkReminderCLIChanges(t, actual["changes"], row["result"])
}

func checkReminderCLIChanges(t *testing.T, actual, source json.RawMessage) {
	t.Helper()

	var events, expected []map[string]json.RawMessage

	decode(t, actual, &events)
	decode(t, source, &expected)

	if len(events) != len(expected) {
		t.Fatal("CLI changed the number of events")
	}

	for index, event := range events {
		before := expected[index]
		if len(event) != 3 || len(before) != 3 {
			t.Fatal("CLI event field inventory changed")
		}

		checkReminderCLIValue(t, event["type"], before["type"])
		checkReminderCLIValue(t, event["reminderID"], before["reminder_id"])

		if string(before["reminder"]) == reminderCLINullValue {
			checkReminderCLIValue(t, event["reminder"], before["reminder"])
		} else {
			checkReminderCLIReminder(t, event["reminder"], before["reminder"])
		}
	}
}
