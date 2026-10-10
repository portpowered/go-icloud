package replay_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const reminderSyncCommand = "reminder-sync"

func checkReminderCLISyncOutcome(t *testing.T, row map[string]json.RawMessage, output []byte, err error) {
	t.Helper()

	if len(row["error"]) != 0 {
		var failure *icloud.ClientError
		if len(output) != 0 || !errors.As(err, &failure) {
			t.Fatal("CLI lost sync error or printed partial data")
		}

		checkReminderCLIFailure(t, row, failure)
		checkReminderCLIPrior(t, row, failure)

		return
	}

	if err != nil {
		t.Fatal(err)
	}

	var (
		actual   map[string]json.RawMessage
		expected string
	)

	decode(t, output, &actual)
	decode(t, row["result"], &expected)

	if len(actual) != 1 {
		t.Fatal("CLI sync result contains unexpected fields")
	}

	checkReminderCLIValue(t, actual[expectedReplaySyncToken], expected)
}

func checkReminderCLIPrior(t *testing.T, row map[string]json.RawMessage, failure *icloud.ClientError) {
	t.Helper()

	var exchanges []replay.Exchange

	decode(t, row[expectedReplayExchanges], &exchanges)

	actual := failure.PriorResponses()
	if len(actual) != len(exchanges)-1 {
		t.Fatal("CLI sync lost prior response evidence")
	}

	for index, metadata := range actual {
		if !reflect.DeepEqual(metadata, referenceResponseMetadata(exchanges[index])) {
			t.Fatal("CLI sync changed prior response metadata")
		}
	}
}
