package replay_test

import (
	"encoding/json"
	"testing"
)

func TestReminderSnapshotCommands(t *testing.T) {
	t.Parallel()
	testReminderCLIInventory(t, "snapshot", expectedReplayReminderSnapshot, 28)
}

func checkReminderCLISnapshotOutcome(t *testing.T, row map[string]json.RawMessage, output []byte, err error) {
	t.Helper()

	if len(row["error"]) != 0 {
		checkReminderCommandOutcome(t, expectedReplayReminderSnapshot, row, output, err)

		return
	}

	if err != nil {
		t.Fatal(err)
	}

	var actual map[string]json.RawMessage

	decode(t, output, &actual)

	if len(actual) != 1 || string(actual[expectedReplayReminders]) == reminderCLINullValue {
		t.Fatal("CLI snapshot contains unexpected fields or null reminders")
	}

	var reminders, expected []json.RawMessage

	decode(t, actual[expectedReplayReminders], &reminders)
	decode(t, row["result"], &expected)

	if len(reminders) != len(expected) {
		t.Fatal("CLI snapshot reminder count differs")
	}

	for index, reminder := range reminders {
		checkReminderCLIReminder(t, reminder, expected[index])
	}
}
