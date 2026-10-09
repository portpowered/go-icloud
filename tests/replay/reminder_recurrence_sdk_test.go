package replay_test

import (
	"encoding/json"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const (
	reminderRecurrenceSourceWeekday = "first_day_of_week"
	reminderRecurrenceSourceCount   = "occurrence_count"
	reminderRecurrenceSourceParent  = "reminder_id"
	reminderRecurrenceSuccess       = "success"
	reminderRecurrenceUpdate        = "update_recurrence_rule"
	reminderRecurrenceParentID      = "reminderID"
	reminderRecurrenceCount         = "occurrenceCount"
	reminderRecurrenceWeekday       = "firstDayOfWeek"
)

func TestReminderRecurrenceSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	for _, operation := range []string{"create", "update", "delete"} {
		for _, outcome := range []string{reminderRecurrenceSuccess, "record-error"} {
			t.Run(operation+"-"+outcome, func(t *testing.T) {
				t.Parallel()
				runReminderRecurrence(t, "fixtures/synthetic/http/reminders-"+operation+"-recurrence-rule-"+outcome+".json")
			})
		}
	}
}

func runReminderRecurrence(t *testing.T, path string) {
	t.Helper()
	row := authReplayObject(t, path)
	scenario := readAccountScenario(t, path)

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, entropy := reminderWriteClient(t, row, transport)

	var operation string

	authReplayDecode(t, row["operation"], &operation)

	result, callErr := callReminderRecurrenceFixture(t, client, row, scenario, operation)
	if len(scenario.Error) != 0 {
		checkReminderWriteFailure(t, scenario, nil, callErr)
	} else {
		if callErr != nil || result == nil {
			t.Fatal(callErr)
		}

		checkReminderRecurrenceProjection(t, row, result, operation)
		checkSDKMetadata(t, result.Responses[0], scenario.Exchanges[0].Response)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}

	if entropy.Len() != 0 {
		t.Fatal("recurrence mutation did not consume declared identities")
	}
}

//nolint:wrapcheck // LIB-05: preserve SDK error classes for replay assertions.
func callReminderRecurrenceFixture(t *testing.T, client *icloud.SDK, row map[string]json.RawMessage,
	scenario accountScenario, operation string,
) (*icloud.ReminderRecurrenceRuleRelationResult, error) {
	t.Helper()

	auth := sdkAccountAuth(scenario.Initial)
	auth.RemindersServiceURL = scenario.Initial.Origin

	var inputs []map[string]json.RawMessage

	authReplayDecode(t, row["inputs"], &inputs)

	var options map[string]json.RawMessage
	if raw := row["keyword_inputs"]; len(raw) != 0 {
		authReplayDecode(t, raw, &options)
	}

	if operation == "create_recurrence_rule" {
		request := new(icloud.CreateReminderRecurrenceRuleRequest)
		authReplayDecode(t, marshalFindMyRecovery(t, sourceRecurrenceOptions(options)), request)
		request.Auth, request.Reminder = auth, reminderUpdateFixture(t, row, scenario).Reminder
		before := marshalFindMyRecovery(t, request)
		result, err := client.CreateReminderRecurrenceRule(t.Context(), *request)
		checkSDKValue(t, request, before)

		return result, err
	}

	index := 0
	if operation == "delete_recurrence_rule" {
		index = 1
	}

	rule := sourceRecurrenceFixture(t, inputs[index]["value"])

	if operation == reminderRecurrenceUpdate {
		request := new(icloud.UpdateReminderRecurrenceRuleRequest)
		authReplayDecode(t, marshalFindMyRecovery(t, sourceRecurrenceOptions(options)), request)
		request.Auth, request.RecurrenceRule = auth, rule
		before := marshalFindMyRecovery(t, request)
		result, err := client.UpdateReminderRecurrenceRule(t.Context(), *request)
		checkSDKValue(t, request, before)

		if result == nil {
			return nil, err
		}

		var reminder icloud.Reminder

		return &icloud.ReminderRecurrenceRuleRelationResult{Reminder: reminder,
			RecurrenceRule: result.RecurrenceRule, Responses: result.Responses}, err
	}

	request := icloud.DeleteReminderRecurrenceRuleRequest{Auth: auth,
		Reminder: reminderUpdateFixture(t, row, scenario).Reminder, RecurrenceRule: rule}
	before := marshalFindMyRecovery(t, request)
	result, err := client.DeleteReminderRecurrenceRule(t.Context(), request)
	checkSDKValue(t, request, before)

	return result, err
}

func sourceRecurrenceOptions(value map[string]json.RawMessage) map[string]json.RawMessage {
	for source, target := range map[string]string{
		reminderRecurrenceSourceCount: reminderRecurrenceCount, reminderRecurrenceSourceWeekday: reminderRecurrenceWeekday,
	} {
		if raw, ok := value[source]; ok {
			value[target] = raw
			delete(value, source)
		}
	}

	return value
}

func sourceRecurrenceFixture(t *testing.T, raw json.RawMessage) icloud.ReminderRecurrenceRule {
	t.Helper()

	var value map[string]json.RawMessage

	authReplayDecode(t, raw, &value)

	value = sourceRecurrenceOptions(value)
	for source, target := range map[string]string{
		reminderRecurrenceSourceParent: reminderRecurrenceParentID, reminderSourceRevisionField: reminderPublicRevisionField,
	} {
		if field, ok := value[source]; ok {
			value[target] = field
			delete(value, source)
		}
	}

	if _, exists := value[reminderPublicRevisionField]; !exists {
		value[reminderPublicRevisionField] = json.RawMessage(`null`)
	}

	var rule icloud.ReminderRecurrenceRule

	authReplayDecode(t, marshalFindMyRecovery(t, value), &rule)

	return rule
}

func checkReminderRecurrenceProjection(t *testing.T, row map[string]json.RawMessage,
	result *icloud.ReminderRecurrenceRuleRelationResult, operation string,
) {
	t.Helper()

	var expected map[string]json.RawMessage

	authReplayDecode(t, row["result"], &expected)

	var arguments []json.RawMessage

	authReplayDecode(t, expected["arguments"], &arguments)

	raw := expected["value"]

	switch operation {
	case reminderRecurrenceUpdate:
		raw = arguments[0]
	case "delete_recurrence_rule":
		raw = arguments[1]
	}

	checkSDKValue(t, result.RecurrenceRule, marshalFindMyRecovery(t, sourceRecurrenceFixture(t, raw)))

	if operation != reminderRecurrenceUpdate {
		checkReminderProjection(t, result.Reminder, arguments[0])
	}

	if len(result.Responses) != 1 {
		t.Fatal("recurrence mutation lost response evidence")
	}
}
