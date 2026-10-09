package replay_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestReminderListsSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/reminders-lists-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 24 {
		t.Fatal("reminder list scenario inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runReminderListsSDK(t, readAccountScenario(t, path))
		})
	}
}

func TestReminderListUnionSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/reminders-list-union-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 13 {
		t.Fatal("reminder list union scenario inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runReminderListsSDK(t, readAccountScenario(t, path))
		})
	}
}

func runReminderListsSDK(t *testing.T, scenario accountScenario) {
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
	result, err := client.ListReminderLists(t.Context(), icloud.ListReminderListsRequest{Auth: auth})

	if len(scenario.Error) != 0 {
		checkReminderListsFailure(t, scenario, result, err)
	} else {
		if err != nil {
			t.Fatal(err)
		}

		checkReminderListProjection(t, result.Lists, scenario.Result)

		if len(result.Responses) != len(scenario.Exchanges) {
			t.Fatal("reminder list response inventory differs")
		}

		for index, exchange := range scenario.Exchanges {
			checkSDKMetadata(t, result.Responses[index], exchange.Response)
		}
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func checkReminderListsFailure(t *testing.T, scenario accountScenario,
	result *icloud.ListReminderListsResult, err error,
) {
	t.Helper()

	var (
		failure  *icloud.ClientError
		expected map[string]string
	)

	decodeErr := json.Unmarshal(scenario.Error, &expected)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}

	kind := icloud.InvalidResponse
	if strings.HasPrefix(expected["message"], "Fetch reminder lists failed") {
		kind = icloud.Provider
	}

	if result != nil || !errors.As(err, &failure) || failure.Kind() != kind {
		t.Fatalf("reminder per-record failure lost its classification: %v", err)
	}

	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response
	if failure.StatusCode() != last.Status || string(failure.ResponseBody()) != string(contractAuthBody(t, last.Body)) {
		t.Fatal("reminder list failure lost exact response evidence")
	}

	checkSDKMetadata(t, icloud.ResponseMetadata{StatusCode: failure.StatusCode(),
		Headers: failure.ResponseHeaders(), CookieScopeURL: failure.CookieScopeURL()}, last)
}

func checkReminderListProjection(t *testing.T, actual []icloud.ReminderList, expected json.RawMessage) {
	t.Helper()

	var lists []map[string]json.RawMessage

	err := json.Unmarshal(expected, &lists)
	if err != nil {
		t.Fatal(err)
	}

	for _, list := range lists {
		for source, destination := range map[string]string{"badge_emblem": "badgeEmblem",
			"sorting_style": "sortingStyle", "is_group": "isGroup", "reminder_ids": "reminderIDs",
			"record_change_tag": "recordChangeTag"} {
			list[destination] = list[source]
			delete(list, source)
		}
	}

	encoded, err := json.Marshal(lists)
	if err != nil {
		t.Fatal(err)
	}

	checkSDKValue(t, actual, encoded)
}
