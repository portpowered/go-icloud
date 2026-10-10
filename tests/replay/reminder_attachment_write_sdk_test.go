package replay_test

import (
	"encoding/json"
	"maps"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestReminderAttachmentWritePortableScenarios(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"create-url-attachment-success", "create-url-attachment-record-error",
		"update-attachment-success", "update-attachment-record-error", "update-image-success",
		"delete-attachment-success", "delete-attachment-record-error", "delete-attachment-empty-id-success",
		"delete-image-success"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runReminderAttachmentWrite(t, replayLiteralFixturesSyntheticHTTPReminders+name+".json")
		})
	}
}

func runReminderAttachmentWrite(t *testing.T, path string) {
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
		attachment icloud.ReminderAttachment
		reminder   *icloud.Reminder
		metadata   []icloud.ResponseMetadata
		callErr    error
	)

	switch operation {
	case "create_url_attachment":
		attachment, reminder, metadata, callErr = replayCreateAttachment(t, client, row, scenario)
	case replayLiteralUpdateAttachment:
		attachment, metadata, callErr = replayUpdateAttachment(t, client, row, scenario)
	case replayLiteralDeleteAttachment:
		attachment, reminder, metadata, callErr = replayDeleteAttachment(t, client, row, scenario)
	default:
		t.Fatal("unknown attachment operation", operation)
	}

	if len(scenario.Error) == 0 {
		if callErr != nil {
			t.Fatal(callErr)
		}

		checkAttachmentWriteProjection(t, row, operation, attachment, reminder)
		checkReminderSyncResponses(t, metadata, scenario.Exchanges)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}

	if entropy.Len() != 0 {
		t.Fatal("attachment write did not consume the declared identities")
	}
}

//nolint:wrapcheck // LIB-05: preserve the SDK failure for exact provider evidence assertions.
func replayCreateAttachment(t *testing.T, client *icloud.SDK, row map[string]json.RawMessage,
	scenario accountScenario,
) (icloud.ReminderAttachment, *icloud.Reminder, []icloud.ResponseMetadata, error) {
	t.Helper()

	var inputs []json.RawMessage

	authReplayDecode(t, row["inputs"], &inputs)
	reminderRow := maps.Clone(row)
	reminderRow["inputs"] = marshalFindMyRecovery(t, inputs[:1])
	base := reminderUpdateFixture(t, reminderRow, scenario)
	request := new(icloud.CreateReminderURLAttachmentRequest)
	request.Auth, request.Reminder = base.Auth, base.Reminder
	authReplayDecode(t, inputs[1], &request.URL)

	if len(row[replayLiteralKeywordInputs]) != 0 {
		authReplayDecode(t, row[replayLiteralKeywordInputs], request)
	}

	before := marshalFindMyRecovery(t, request)
	result, err := client.CreateReminderURLAttachment(t.Context(), *request)
	checkSDKValue(t, request, before)

	if len(scenario.Error) != 0 {
		checkReminderWriteFailure(t, scenario, result, err)
	}

	if result == nil {
		return icloud.ReminderAttachment{}, nil, nil, err
	}

	return result.Attachment, &result.Reminder, result.Responses, err
}

//nolint:wrapcheck // LIB-05: preserve the SDK failure for exact provider evidence assertions.
func replayUpdateAttachment(t *testing.T, client *icloud.SDK, row map[string]json.RawMessage,
	scenario accountScenario,
) (icloud.ReminderAttachment, []icloud.ResponseMetadata, error) {
	t.Helper()

	request := new(icloud.UpdateReminderAttachmentRequest)
	request.Auth = sdkAccountAuth(scenario.Initial)
	request.Auth.RemindersServiceURL = scenario.Initial.Origin
	request.Attachment = attachmentFixture(t, row, 0)

	var options map[string]json.RawMessage

	authReplayDecode(t, row[replayLiteralKeywordInputs], &options)
	renameReminderRelated(t, options)
	authReplayDecode(t, marshalFindMyRecovery(t, options), request)
	before := marshalFindMyRecovery(t, request)
	result, err := client.UpdateReminderAttachment(t.Context(), *request)
	checkSDKValue(t, request, before)

	if len(scenario.Error) != 0 {
		checkReminderWriteFailure(t, scenario, result, err)
	}

	if result == nil {
		return icloud.ReminderAttachment{}, nil, err
	}

	return result.Attachment, result.Responses, err
}

//nolint:wrapcheck // LIB-05: preserve the SDK failure for exact provider evidence assertions.
func replayDeleteAttachment(t *testing.T, client *icloud.SDK, row map[string]json.RawMessage,
	scenario accountScenario,
) (icloud.ReminderAttachment, *icloud.Reminder, []icloud.ResponseMetadata, error) {
	t.Helper()
	base := reminderUpdateFixture(t, row, scenario)
	request := icloud.DeleteReminderAttachmentRequest{Auth: base.Auth, Reminder: base.Reminder,
		Attachment: attachmentFixture(t, row, 1)}
	before := marshalFindMyRecovery(t, request)
	result, err := client.DeleteReminderAttachment(t.Context(), request)
	checkSDKValue(t, request, before)

	if len(scenario.Error) != 0 {
		checkReminderWriteFailure(t, scenario, result, err)
	}

	if result == nil {
		return icloud.ReminderAttachment{}, nil, nil, err
	}

	return result.Attachment, &result.Reminder, result.Responses, err
}

func attachmentFixture(t *testing.T, row map[string]json.RawMessage, index int) icloud.ReminderAttachment {
	t.Helper()

	var inputs []map[string]json.RawMessage

	authReplayDecode(t, row["inputs"], &inputs)

	var (
		value map[string]json.RawMessage
		model string
	)

	authReplayDecode(t, inputs[index]["value"], &value)
	authReplayDecode(t, inputs[index]["$model"], &model)
	renameReminderRelated(t, value)

	if _, exists := value["uti"]; !exists {
		if model == "URLAttachment" {
			value["uti"] = json.RawMessage(`"public.url"`)
		} else {
			value["uti"] = json.RawMessage(`"Image"`)
		}
	}

	if model == "ImageAttachment" {
		if _, exists := value[replayExpectedFileAssetURL]; !exists {
			value[replayExpectedFileAssetURL] = json.RawMessage(`""`)
		}
	}

	var attachment icloud.ReminderAttachment

	authReplayDecode(t, marshalFindMyRecovery(t, value), &attachment)

	return attachment
}

func checkAttachmentWriteProjection(t *testing.T, row map[string]json.RawMessage, operation string,
	attachment icloud.ReminderAttachment, reminder *icloud.Reminder,
) {
	t.Helper()

	var expected struct {
		Value     json.RawMessage   `json:"value"`
		Arguments []json.RawMessage `json:"arguments"`
	}

	authReplayDecode(t, row["result"], &expected)

	value := expected.Value
	if operation == replayLiteralUpdateAttachment {
		value = expected.Arguments[0]
	} else {
		checkReminderProjection(t, *reminder, expected.Arguments[0])

		if operation == replayLiteralDeleteAttachment {
			value = expected.Arguments[1]
		}
	}

	var object map[string]json.RawMessage

	authReplayDecode(t, value, &object)
	renameReminderRelated(t, object)
	checkSDKValue(t, attachment, marshalFindMyRecovery(t, object))
}
