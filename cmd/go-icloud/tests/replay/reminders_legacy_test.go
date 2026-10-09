package replay_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

const legacyRemindersCommand = "reminder-legacy-snapshot"

func TestLegacyRemindersCommand(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("../../../../tests/replay/fixtures/synthetic/http/reminders-legacy-*.json")
	if err != nil || len(paths) != 9 {
		t.Fatal("legacy Reminders CLI inventory changed", err)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runReminderCommand(t, legacyRemindersCommand, readObject(t, path))
		})
	}
}

func checkLegacyReminderCLIOutcome(t *testing.T, row map[string]json.RawMessage, output []byte, err error) {
	t.Helper()

	if len(row["error"]) != 0 {
		var failure *icloud.ClientError
		if len(output) != 0 || !errors.As(err, &failure) {
			t.Fatal("legacy Reminders CLI printed partial output or lost typed failure")
		}

		checkReminderCLIFailure(t, row, failure)

		return
	}

	if err != nil {
		t.Fatal(err)
	}

	var expected any

	decode(t, row["result"], &expected)
	checkReminderCLIValue(t, output, expected)
}
