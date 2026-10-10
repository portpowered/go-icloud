package replay_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const (
	reminderSourceHashtagIDs    = "hashtag_ids"
	reminderSourceAlarmIDs      = "alarm_ids"
	reminderSourceAttachmentIDs = "attachment_ids"
	reminderSourceRecurrenceIDs = "recurrence_rule_ids"
	reminderSourceCreated       = "created"
	reminderSourceParentID      = "parent_reminder_id"
	reminderSourceCompletedDate = "completed_date"
)

func TestReminderWriteSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"create-basic", "create-completed", "create-dated-child", "create-lookup-empty",
		"create-record-error", "update-basic", "update-completed", "update-dated-child", "update-record-error"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runReminderWrite(t, replayLiteralFixturesSyntheticHTTPReminders+name+".json")
		})
	}
}

func reminderWriteClient(t *testing.T, row map[string]json.RawMessage,
	transport *replay.HTTPTransport,
) (*icloud.SDK, *bytes.Buffer) {
	t.Helper()

	var entropy struct {
		UUIDs []string `json:"uuid4"`
		//nolint:tagliatelle // LIB-05: reference fixture spelling.
		Seconds int64 `json:"unix_seconds"`
	}

	authReplayDecode(t, row["entropy"], &entropy)

	var random bytes.Buffer

	for _, identity := range entropy.UUIDs {
		data, err := hex.DecodeString(strings.ReplaceAll(identity, "-", ""))
		if err != nil {
			t.Fatal(err)
		}

		random.Write(data)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport), icloud.WithRandomSource(&random),
		icloud.WithClock(func() time.Time { return time.Unix(entropy.Seconds, 0) }))
	if err != nil {
		t.Fatal(err)
	}

	return client, &random
}

func runReminderWrite(t *testing.T, path string) {
	t.Helper()
	row := authReplayObject(t, path)
	scenario := readAccountScenario(t, path)

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, entropy := reminderWriteClient(t, row, transport)

	var operation string

	authReplayDecode(t, row[replayExpectedOperation], &operation)

	var (
		result  *icloud.ReminderMutationResult
		callErr error
	)

	if operation == "create" {
		request := reminderCreateFixture(t, row, scenario)
		before := marshalFindMyRecovery(t, request)
		result, callErr = client.CreateReminder(t.Context(), request)
		checkSDKValue(t, request, before)
	} else {
		request := reminderUpdateFixture(t, row, scenario)
		before := marshalFindMyRecovery(t, request)
		result, callErr = client.UpdateReminder(t.Context(), request)
		checkSDKValue(t, request, before)
	}

	checkReminderWriteOutcome(t, row, scenario, operation, result, callErr)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}

	if entropy.Len() != 0 {
		t.Fatal("mutation did not consume declared identities")
	}
}

func reminderCreateFixture(t *testing.T, row map[string]json.RawMessage,
	scenario accountScenario,
) icloud.CreateReminderRequest {
	t.Helper()

	var inputs []string

	authReplayDecode(t, row["inputs"], &inputs)

	var options map[string]json.RawMessage
	if len(row[replayLiteralKeywordInputs]) != 0 {
		authReplayDecode(t, row[replayLiteralKeywordInputs], &options)
	}

	request := new(icloud.CreateReminderRequest)
	request.Auth = sdkAccountAuth(scenario.Initial)
	request.Auth.RemindersServiceURL = scenario.Initial.Origin

	request.ListID, request.Title = inputs[0], inputs[1]
	if len(inputs) > 2 {
		request.Description = inputs[2]
	}

	if options != nil {
		body := marshalFindMyRecovery(t, sourceReminderFields(t, options))
		auth := request.Auth
		authReplayDecode(t, body, request)
		request.Auth = auth
		request.ListID, request.Title = inputs[0], inputs[1]
	}

	return *request
}

func reminderUpdateFixture(t *testing.T, row map[string]json.RawMessage,
	scenario accountScenario,
) icloud.UpdateReminderRequest {
	t.Helper()

	var inputs []map[string]json.RawMessage

	authReplayDecode(t, row["inputs"], &inputs)

	var value map[string]json.RawMessage

	authReplayDecode(t, inputs[0]["value"], &value)

	for _, key := range []string{
		reminderSourceCompletedDate, replayLiteralDueDate, replayExpectedStartDate,
		reminderSourceCreated, replayLiteralModified,
		reminderSourceParentID, reminderSourceRevisionField, replayLiteralTimeZone} {
		if _, ok := value[key]; !ok {
			value[key] = json.RawMessage(`null`)
		}
	}

	for _, key := range []string{reminderSourceAlarmIDs, reminderSourceAttachmentIDs,
		reminderSourceHashtagIDs, reminderSourceRecurrenceIDs} {
		if _, ok := value[key]; !ok {
			value[key] = json.RawMessage(`[]`)
		}
	}

	var reminder icloud.Reminder

	authReplayDecode(t, marshalFindMyRecovery(t, sourceReminderFields(t, value)), &reminder)

	auth := sdkAccountAuth(scenario.Initial)
	auth.RemindersServiceURL = scenario.Initial.Origin

	return icloud.UpdateReminderRequest{Auth: auth, Reminder: reminder}
}

func sourceReminderFields(t *testing.T, value map[string]json.RawMessage) map[string]json.RawMessage {
	t.Helper()

	for source, target := range map[string]string{
		"list_id": "listID", "desc": "description", reminderSourceCompletedDate: replayLiteralCompletedDate,
		replayLiteralDueDate:    "dueDate",
		replayExpectedStartDate: replayLiteralStartDate, "all_day": "allDay", replayLiteralTimeZone: "timeZone",
		reminderSourceAlarmIDs:   "alarmIDs",
		reminderSourceHashtagIDs: "hashtagIDs", reminderSourceAttachmentIDs: "attachmentIDs",
		reminderSourceRecurrenceIDs: "recurrenceRuleIDs",
		reminderSourceParentID:      "parentReminderID", reminderSourceRevisionField: reminderPublicRevisionField} {
		if raw, ok := value[source]; ok {
			value[target] = raw
			delete(value, source)
		}
	}

	for _, key := range []string{
		replayLiteralCompletedDate, "dueDate", replayLiteralStartDate, reminderSourceCreated, replayLiteralModified,
	} {
		raw := value[key]
		if len(raw) == 0 || string(raw) == reminderChangeNullValue {
			continue
		}

		var text string

		if raw[0] == '{' {
			var wrapped map[string]string

			authReplayDecode(t, raw, &wrapped)
			text = wrapped["$datetime"]
		} else {
			authReplayDecode(t, raw, &text)
		}

		if !strings.ContainsAny(text[len("2000-01-01T00:00:00"):], "Z+-") {
			text += "Z"
		}

		instant, err := time.Parse(time.RFC3339Nano, text)
		if err != nil {
			t.Fatal(err)
		}

		value[key] = marshalFindMyRecovery(t, instant)
	}

	return value
}

func checkReminderWriteOutcome(t *testing.T, row map[string]json.RawMessage, scenario accountScenario,
	operation string, result *icloud.ReminderMutationResult, callErr error,
) {
	t.Helper()

	if len(scenario.Error) != 0 {
		checkReminderWriteFailure(t, scenario, result, callErr)

		return
	}

	if result == nil || callErr != nil {
		t.Fatal(callErr, errors.Unwrap(callErr))
	}

	expected := row["result"]

	if operation == "update" {
		var envelope struct {
			Arguments []json.RawMessage `json:"arguments"`
		}

		authReplayDecode(t, expected, &envelope)
		expected = envelope.Arguments[0]
	}

	checkReminderProjection(t, result.Reminder, expected)

	if len(result.Responses) != len(scenario.Exchanges) {
		t.Fatal("mutation lost response evidence")
	}

	for index, response := range result.Responses {
		checkSDKMetadata(t, response, scenario.Exchanges[index].Response)
	}
}

func checkReminderWriteFailure(t *testing.T, scenario accountScenario,
	result *icloud.ReminderMutationResult, callErr error,
) {
	t.Helper()

	var failure *icloud.ClientError

	kind := icloud.Provider
	if strings.Contains(string(scenario.Error), replayLiteralLookupError) {
		kind = icloud.NotFound
	}

	if result != nil || !errors.As(callErr, &failure) || failure.Kind() != kind {
		t.Fatal("incorrect mutation failure", callErr)
	}

	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response
	if !bytes.Equal(failure.ResponseBody(), contractAuthBody(t, last.Body)) {
		t.Fatal("mutation failure lost response body")
	}

	if len(failure.PriorResponses()) != len(scenario.Exchanges)-1 {
		t.Fatal("mutation failure lost prior response evidence")
	}

	checkSDKMetadata(t, icloud.ResponseMetadata{StatusCode: failure.StatusCode(),
		Headers: failure.ResponseHeaders(), CookieScopeURL: failure.CookieScopeURL()}, last)

	for index, response := range failure.PriorResponses() {
		checkSDKMetadata(t, response, scenario.Exchanges[index].Response)
	}
}
