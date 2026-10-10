package replay_test

import (
	"encoding/json"
	"maps"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const reminderHashtagSuccess = "success"

func TestReminderHashtagSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	for _, operation := range []string{"create", "update", "delete"} {
		for _, outcome := range []string{reminderHashtagSuccess, replayLiteralRecordError} {
			t.Run(operation+"/"+outcome, func(t *testing.T) {
				t.Parallel()
				runReminderHashtag(t, replayLiteralFixturesSyntheticHTTPReminders+operation+"-hashtag-"+outcome+".json")
			})
		}
	}
}

func runReminderHashtag(t *testing.T, path string) {
	t.Helper()
	row := authReplayObject(t, path)
	scenario := readAccountScenario(t, path)

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, entropy := reminderWriteClient(t, row, transport)
	result, callErr := callReminderHashtag(t, row, scenario, client)
	checkReminderHashtagOutcome(t, row, scenario, result, callErr)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}

	if entropy.Len() != 0 {
		t.Fatal("hashtag write did not consume declared identities")
	}
}

//nolint:wrapcheck // LIB-05: retain client errors unchanged for complete evidence assertions.
func callReminderHashtag(t *testing.T, row map[string]json.RawMessage,
	scenario accountScenario, client *icloud.SDK,
) (*icloud.ReminderHashtagRelationResult, error) {
	t.Helper()

	var (
		operation string
		inputs    []json.RawMessage
	)

	authReplayDecode(t, row[replayExpectedOperation], &operation)
	authReplayDecode(t, row["inputs"], &inputs)

	auth := sdkAccountAuth(scenario.Initial)
	auth.RemindersServiceURL = scenario.Initial.Origin

	if operation == replayLiteralUpdateHashtag {
		request := new(icloud.UpdateReminderHashtagRequest)
		request.Auth = auth
		request.Hashtag = reminderHashtagFixture(t, inputs[0])
		authReplayDecode(t, inputs[1], &request.Name)
		before := marshalFindMyRecovery(t, request)
		result, err := client.UpdateReminderHashtag(t.Context(), *request)
		checkSDKValue(t, request, before)

		if result == nil {
			return nil, err
		}

		converted := new(icloud.ReminderHashtagRelationResult)
		converted.Hashtag, converted.Responses = result.Hashtag, result.Responses

		return converted, err
	}

	reminderRow := maps.Clone(row)
	reminderRow["inputs"] = marshalFindMyRecovery(t, inputs[:1])
	reminder := reminderUpdateFixture(t, reminderRow, scenario).Reminder

	if operation == replayLiteralCreateHashtag {
		request := new(icloud.CreateReminderHashtagRequest)
		request.Auth, request.Reminder = auth, reminder
		authReplayDecode(t, inputs[1], &request.Name)
		before := marshalFindMyRecovery(t, request)
		result, err := client.CreateReminderHashtag(t.Context(), *request)
		checkSDKValue(t, request, before)

		return result, err
	}

	request := icloud.DeleteReminderHashtagRequest{Auth: auth, Reminder: reminder,
		Hashtag: reminderHashtagFixture(t, inputs[1])}
	before := marshalFindMyRecovery(t, request)
	result, err := client.DeleteReminderHashtag(t.Context(), request)
	checkSDKValue(t, request, before)

	return result, err
}

func reminderHashtagFixture(t *testing.T, raw json.RawMessage) icloud.ReminderHashtag {
	t.Helper()

	var wrapper struct {
		Value map[string]json.RawMessage `json:"value"`
	}

	authReplayDecode(t, raw, &wrapper)
	fields := sourceReminderFields(t, wrapper.Value)
	fields["reminderID"] = fields[reminderLocationSourceReminderID]
	delete(fields, reminderLocationSourceReminderID)

	if _, exists := fields[reminderSourceCreated]; !exists {
		fields[reminderSourceCreated] = json.RawMessage(`null`)
	}

	if _, exists := fields[reminderPublicRevisionField]; !exists {
		fields[reminderPublicRevisionField] = json.RawMessage(`null`)
	}

	var hashtag icloud.ReminderHashtag

	authReplayDecode(t, marshalFindMyRecovery(t, fields), &hashtag)

	return hashtag
}

func checkReminderHashtagOutcome(t *testing.T, row map[string]json.RawMessage, scenario accountScenario,
	result *icloud.ReminderHashtagRelationResult, callErr error,
) {
	t.Helper()

	if len(scenario.Error) != 0 {
		if result != nil {
			t.Fatal("rejected hashtag write returned updated snapshots")
		}

		checkReminderWriteFailure(t, scenario, nil, callErr)

		return
	}

	if result == nil || callErr != nil {
		t.Fatal("hashtag mutation failed", callErr)
	}

	var (
		operation string
		expected  struct {
			Value     json.RawMessage   `json:"value"`
			Arguments []json.RawMessage `json:"arguments"`
		}
	)

	authReplayDecode(t, row[replayExpectedOperation], &operation)
	authReplayDecode(t, row["result"], &expected)

	child := expected.Value
	if operation != replayLiteralCreateHashtag {
		child = expected.Arguments[len(expected.Arguments)-1]
	}

	checkReminderRelatedProjection(t, map[string]icloud.ReminderHashtag{"child": result.Hashtag},
		marshalFindMyRecovery(t, map[string]json.RawMessage{"child": child}))

	if operation != replayLiteralUpdateHashtag {
		checkReminderProjection(t, result.Reminder, expected.Arguments[0])
	}

	if len(result.Responses) != len(scenario.Exchanges) {
		t.Fatal("hashtag mutation lost response evidence")
	}

	for index, response := range result.Responses {
		checkSDKMetadata(t, response, scenario.Exchanges[index].Response)
	}
}
