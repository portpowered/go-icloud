package replay_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const (
	reminderLocationPublicReminderID = "reminderID"
	reminderLocationPublicTriggerID  = "triggerID"
	reminderLocationSourceReminderID = "reminder_id"
	reminderLocationSourceTriggerID  = "trigger_id"

	reminderLocationSourceAlarmUID    = "alarm_uid"
	reminderLocationSourceAlarmID     = "alarm_id"
	reminderLocationSourceLocationUID = "location_uid"

	reminderLocationSuccessFixture    = "success"
	reminderLocationPublicAlarmUID    = "alarmUID"
	reminderLocationPublicAlarmID     = "alarmID"
	reminderLocationPublicLocationUID = "locationUID"
)

func TestReminderLocationSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	for _, name := range []string{reminderLocationSuccessFixture, "record-error"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runReminderLocation(t, "fixtures/synthetic/http/reminders-add-location-trigger-"+name+".json")
		})
	}
}

func runReminderLocation(t *testing.T, path string) {
	t.Helper()
	row := authReplayObject(t, path)
	scenario := readAccountScenario(t, path)

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, entropy := reminderWriteClient(t, row, transport)
	base := reminderUpdateFixture(t, row, scenario)
	request := new(icloud.AddReminderLocationTriggerRequest)
	authReplayDecode(t, row["keyword_inputs"], request)
	request.Auth, request.Reminder = base.Auth, base.Reminder
	before := marshalFindMyRecovery(t, request)
	result, callErr := client.AddReminderLocationTrigger(t.Context(), *request)
	checkSDKValue(t, request, before)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}

	if len(scenario.Error) != 0 {
		if result != nil {
			t.Fatal("location rejection returned success")
		}

		checkReminderWriteFailure(t, scenario, nil, callErr)
	} else {
		if result == nil || callErr != nil {
			t.Fatal(callErr, errors.Unwrap(callErr))
		}

		checkReminderLocationOutcome(t, row, scenario, result)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}

	if entropy.Len() != 0 {
		t.Fatal("location mutation did not consume declared identities")
	}
}

func checkReminderLocationOutcome(t *testing.T, row map[string]json.RawMessage, scenario accountScenario,
	result *icloud.AddReminderLocationTriggerResult,
) {
	t.Helper()

	var expected struct {
		Value     []map[string]json.RawMessage `json:"value"`
		Arguments []json.RawMessage            `json:"arguments"`
	}

	authReplayDecode(t, row["result"], &expected)
	checkReminderProjection(t, result.Reminder, expected.Arguments[0])

	for _, value := range expected.Value {
		for source, target := range map[string]string{
			reminderLocationSourceAlarmUID:    reminderLocationPublicAlarmUID,
			reminderLocationSourceReminderID:  reminderLocationPublicReminderID,
			reminderLocationSourceTriggerID:   reminderLocationPublicTriggerID,
			reminderLocationSourceAlarmID:     reminderLocationPublicAlarmID,
			reminderLocationSourceLocationUID: reminderLocationPublicLocationUID,
			reminderSourceRevisionField:       reminderPublicRevisionField} {
			if raw, ok := value[source]; ok {
				value[target] = raw
				delete(value, source)
			}
		}
	}

	checkSDKValue(t, result.Alarm, marshalFindMyRecovery(t, expected.Value[0]))

	var expectedTrigger icloud.ReminderLocationTrigger

	authReplayDecode(t, marshalFindMyRecovery(t, expected.Value[1]), &expectedTrigger)
	checkSDKValue(t, result.Trigger, marshalFindMyRecovery(t, expectedTrigger))

	if len(result.Responses) != 1 {
		t.Fatal("location mutation lost response evidence")
	}

	checkSDKMetadata(t, result.Responses[0], scenario.Exchanges[0].Response)
}
