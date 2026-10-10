package replay_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

type reminderRelatedScenario struct {
	accountScenario

	Operation string                 `json:"operation"`
	Inputs    []reminderRelatedInput `json:"inputs"`
}

type reminderRelatedInput struct {
	Value map[string]json.RawMessage `json:"value"`
}

func TestReminderRelatedSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/reminders-related-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 46 {
		t.Fatal("related lookup scenario inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			data, readErr := os.ReadFile(filepath.Clean(path))
			if readErr != nil {
				t.Fatal(readErr)
			}

			var scenario reminderRelatedScenario

			decodeErr := json.Unmarshal(data, &scenario)
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}

			runReminderRelatedSDK(t, scenario)
		})
	}
}

func runReminderRelatedSDK(t *testing.T, scenario reminderRelatedScenario) {
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

	actual, responses, err := callReminderRelated(t, client, auth, scenario)
	if len(scenario.Error) == 0 {
		if err != nil {
			t.Fatal(err)
		}

		checkReminderRelatedArray(t, actual, scenario.Result)
		checkReminderSyncResponses(t, responses, scenario.Exchanges)
	} else {
		checkReminderRelatedFailure(t, scenario, actual, err)
	}

	consumeErr := transport.AssertConsumed()
	if consumeErr != nil {
		t.Fatal(consumeErr)
	}
}

func callReminderRelated(t *testing.T, client icloud.Client, auth icloud.AuthContext,
	scenario reminderRelatedScenario,
) (any, []icloud.ResponseMetadata, error) {
	t.Helper()

	key := map[string]string{
		replayExpectedTagsFor: reminderSourceHashtagIDs, replayExpectedAttachmentsFor: reminderSourceAttachmentIDs,
		replayExpectedRecurrenceRulesFor: reminderSourceRecurrenceIDs,
		"alarms_for":                     reminderSourceAlarmIDs}[scenario.Operation]

	var ids []string

	err := json.Unmarshal(scenario.Inputs[0].Value[key], &ids)
	if err != nil {
		t.Fatal(err)
	}

	switch scenario.Operation {
	case replayExpectedTagsFor:
		value, err := client.ListReminderTags(t.Context(), icloud.ListReminderTagsRequest{Auth: auth, IDs: ids})
		if err != nil {
			return nil, nil, fmt.Errorf(replayExpectedRelatedCallFormat, err)
		}

		return value.Items, value.Responses, nil
	case replayExpectedAttachmentsFor:
		value, err := client.ListReminderAttachments(t.Context(), icloud.ListReminderAttachmentsRequest{Auth: auth, IDs: ids})
		if err != nil {
			return nil, nil, fmt.Errorf(replayExpectedRelatedCallFormat, err)
		}

		return value.Items, value.Responses, nil
	case replayExpectedRecurrenceRulesFor:
		value, err := client.ListReminderRecurrenceRules(t.Context(),
			icloud.ListReminderRecurrenceRulesRequest{Auth: auth, IDs: ids})
		if err != nil {
			return nil, nil, fmt.Errorf(replayExpectedRelatedCallFormat, err)
		}

		return value.Items, value.Responses, nil
	default:
		value, err := client.ListReminderAlarms(t.Context(), icloud.ListReminderAlarmsRequest{Auth: auth, IDs: ids})
		if err != nil {
			return nil, nil, fmt.Errorf(replayExpectedRelatedCallFormat, err)
		}

		return value.Items, value.Responses, nil
	}
}

func checkReminderRelatedFailure(t *testing.T, scenario reminderRelatedScenario, actual any, err error) {
	t.Helper()

	var expected map[string]string

	decodeErr := json.Unmarshal(scenario.Error, &expected)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}

	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response

	kind := reminderSyncFailureKind(last.Status)

	if last.Status == 200 {
		kind = icloud.Provider
		if strings.Contains(expected["message"], "validation failed") {
			kind = icloud.InvalidResponse
		}
	}

	var failure *icloud.ClientError
	if actual != nil || !errors.As(err, &failure) || failure.Kind() != kind {
		t.Fatalf("related failure classification: %v", err)
	}

	if string(failure.ResponseBody()) != string(contractAuthBody(t, last.Body)) {
		t.Fatal("related failure body differs")
	}

	checkSDKMetadata(t, icloud.ResponseMetadata{StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders(),
		CookieScopeURL: failure.CookieScopeURL()}, last)
	checkReminderSyncResponses(t, failure.PriorResponses(), scenario.Exchanges[:len(scenario.Exchanges)-1])
}

func checkReminderRelatedArray(t *testing.T, actual any, source json.RawMessage) {
	t.Helper()

	var rows []map[string]json.RawMessage

	err := json.Unmarshal(source, &rows)
	if err != nil {
		t.Fatal(err)
	}

	for _, row := range rows {
		for _, name := range []string{"alarm", "trigger"} {
			raw, exists := row[name]
			if !exists || string(raw) == reminderChangeNullValue {
				continue
			}

			var nested map[string]json.RawMessage

			err = json.Unmarshal(raw, &nested)
			if err != nil {
				t.Fatal(err)
			}

			renameReminderRelated(t, nested)

			encoded, err := json.Marshal(nested)
			if err != nil {
				t.Fatal(err)
			}

			row[name] = encoded
		}

		renameReminderRelated(t, row)
	}

	encoded, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}

	checkSDKValue(t, actual, encoded)
}

func renameReminderRelated(t *testing.T, row map[string]json.RawMessage) {
	t.Helper()

	for source, target := range map[string]string{
		reminderLocationSourceReminderID:  reminderLocationPublicReminderID,
		reminderLocationSourceAlarmID:     "alarmID",
		reminderLocationSourceAlarmUID:    reminderLocationPublicAlarmUID,
		reminderLocationSourceTriggerID:   reminderLocationPublicTriggerID,
		reminderLocationSourceLocationUID: reminderLocationPublicLocationUID,
		replayExpectedFileAssetUrl:        replayExpectedFileAssetURL,
		"file_size":                       replayExpectedFileSize,
		reminderRecurrenceSourceCount:     reminderRecurrenceCount,
		reminderRecurrenceSourceWeekday:   reminderRecurrenceWeekday,
		reminderSourceRevisionField:       reminderPublicRevisionField,
	} {
		if raw, exists := row[source]; exists {
			row[target] = raw
			delete(row, source)
		}
	}

	normalizeReminderRelatedValues(t, row)
}
