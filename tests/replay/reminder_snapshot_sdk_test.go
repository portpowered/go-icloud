package replay_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

type reminderSnapshotScenario struct {
	accountScenario

	Inputs []string `json:"inputs"`
}

func TestReminderSnapshotSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/reminders-snapshot-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 28 {
		t.Fatal("reminder snapshot inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(filepath.Clean(path))
			if err != nil {
				t.Fatal(err)
			}

			var scenario reminderSnapshotScenario

			err = json.Unmarshal(data, &scenario)
			if err != nil {
				t.Fatal(err)
			}

			runReminderSnapshotSDK(t, scenario)
		})
	}
}

func runReminderSnapshotSDK(t *testing.T, scenario reminderSnapshotScenario) {
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

	var list *string
	if len(scenario.Inputs) != 0 {
		list = &scenario.Inputs[0]
	}

	result, err := client.ListReminderSnapshot(t.Context(), icloud.ListReminderSnapshotRequest{Auth: auth, ListID: list})
	if len(scenario.Error) != 0 {
		if result != nil {
			t.Fatal("failed snapshot returned a partial result")
		}

		checkReminderQueryFailure(t, reminderQueryScenario{accountScenario: scenario.accountScenario,
			Inputs: nil, Keywords: reminderQueryKeywords{IncludeCompleted: nil, ResultsLimit: nil}}, nil, err)
	} else {
		if err != nil {
			t.Fatal(err)
		}

		checkReminderSnapshotProjection(t, result.Reminders, scenario.Result)
		checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func checkReminderSnapshotProjection(t *testing.T, reminders []icloud.Reminder, expected json.RawMessage) {
	t.Helper()

	var values []json.RawMessage

	err := json.Unmarshal(expected, &values)
	if err != nil {
		t.Fatal(err)
	}

	if len(reminders) != len(values) {
		t.Fatal("snapshot result size differs")
	}

	for index, value := range values {
		checkReminderProjection(t, reminders[index], value)
	}
}
