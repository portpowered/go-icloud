package replay_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

type sourceReminderChange struct {
	Type       string          `json:"type"`
	ReminderID string          `json:"reminder_id"`
	Reminder   json.RawMessage `json:"reminder"`
}

func checkReminderChangesProjection(t *testing.T, actual []icloud.ReminderChangeEvent, source json.RawMessage) {
	t.Helper()

	var expected []sourceReminderChange

	err := json.Unmarshal(source, &expected)
	if err != nil {
		t.Fatal(err)
	}

	if actual == nil || len(actual) != len(expected) {
		t.Fatal("change event inventory differs")
	}

	for index, event := range actual {
		value := expected[index]
		if string(event.Type) != value.Type || event.ReminderID != value.ReminderID {
			t.Fatal("change event identity or order differs")
		}

		if string(value.Reminder) == "null" {
			if !event.Reminder.IsNull() {
				t.Fatal("tombstone reminder is not explicitly null")
			}

			continue
		}

		reminder, err := event.Reminder.Get()
		if err != nil {
			t.Fatal(err)
		}

		checkReminderProjection(t, reminder, value.Reminder)
	}
}

func checkReminderChangesFailure(t *testing.T, scenario reminderChangesScenario,
	result *icloud.ListReminderChangesResult, err error,
) {
	t.Helper()

	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response
	kind := reminderSyncFailureKind(last.Status)

	var expected map[string]string

	decodeErr := json.Unmarshal(scenario.Error, &expected)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}

	if strings.HasPrefix(expected["message"], "Invalid JSON") ||
		strings.HasPrefix(expected["message"], "Changes response validation") {
		kind = icloud.InvalidResponse
	}

	var failure *icloud.ClientError
	if result != nil || !errors.As(err, &failure) || failure.Kind() != kind {
		t.Fatalf("change failure classification: %v", err)
	}

	if !bytes.Equal(failure.ResponseBody(), contractAuthBody(t, last.Body)) {
		t.Fatal("change failure body differs")
	}

	checkSDKMetadata(t, icloud.ResponseMetadata{StatusCode: failure.StatusCode(),
		Headers: failure.ResponseHeaders(), CookieScopeURL: failure.CookieScopeURL()}, last)
	checkReminderSyncResponses(t, failure.PriorResponses(), scenario.Exchanges[:len(scenario.Exchanges)-1])
}
