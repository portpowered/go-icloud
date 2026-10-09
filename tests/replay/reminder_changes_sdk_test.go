package replay_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

type reminderChangesScenario struct {
	accountScenario

	//nolint:tagliatelle // LIB-05: portable initial-input spelling.
	Keywords reminderChangesKeywords `json:"keyword_inputs"`
}

type reminderChangesKeywords struct {
	Since *string `json:"since"`
}

func TestReminderChangesSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/reminders-changes-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 36 {
		t.Fatal("reminder changes scenario inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(filepath.Clean(path))
			if err != nil {
				t.Fatal(err)
			}

			var scenario reminderChangesScenario

			err = json.Unmarshal(data, &scenario)
			if err != nil {
				t.Fatal(err)
			}

			runReminderChangesSDK(t, scenario)
		})
	}
}

func runReminderChangesSDK(t *testing.T, scenario reminderChangesScenario) {
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

	result, err := client.ListReminderChanges(t.Context(),
		icloud.ListReminderChangesRequest{Auth: auth, Since: scenario.Keywords.Since})
	if len(scenario.Error) != 0 {
		checkReminderChangesFailure(t, scenario, result, err)
	} else {
		if err != nil {
			t.Fatal(err)
		}

		checkReminderChangesProjection(t, result.Changes, scenario.Result)
		checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}
