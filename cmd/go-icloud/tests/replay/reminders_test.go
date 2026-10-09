package replay_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const reminderCommand = "reminder"
const reminderCLIAttachmentsField = "attachments"
const reminderCLIAttachmentsCommand = "reminder-attachments"
const reminderCLIRecurrenceCommand = "reminder-recurrence-rules"
const reminderCLITagsCommand = "reminder-tags"
const reminderCLIAlarmsField = "alarms"
const reminderCLIAlarmsCommand = "reminder-alarms"

func TestReminderCommands(t *testing.T) {
	t.Parallel()

	for prefix, operation := range map[string]string{"zones": "reminder-zones",
		"lists": "reminder-lists", "get": reminderCommand, "sync": reminderSyncCommand,
		"changes": reminderChangesCommand} {
		paths, err := filepath.Glob("../../../../tests/replay/fixtures/synthetic/http/reminders-" + prefix + "-*.json")
		if err != nil {
			t.Fatal(err)
		}

		want := 10
		if prefix == "lists" {
			want = 24
		}

		if prefix == "get" {
			want = 19
		}

		if prefix == "sync" {
			want = 53
		}

		if prefix == "changes" {
			want = 72
		}

		if len(paths) != want {
			t.Fatal("CLI reminder scenario inventory changed")
		}

		for _, path := range paths {
			t.Run(filepath.Base(path), func(t *testing.T) {
				t.Parallel()
				runReminderCommand(t, operation, readObject(t, path))
			})
		}
	}
}

func runReminderCommand(t *testing.T, operation string, row map[string]json.RawMessage) {
	t.Helper()

	var exchanges []replay.Exchange

	decode(t, row["exchanges"], &exchanges)

	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	auth := fixtureAuth(t, row["initial_state"])
	session := filepath.Join(t.TempDir(), "session.json")

	//nolint:gosec // Only synthetic credentials are marshaled for the private replay session.
	encoded, err := json.Marshal(auth)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(session, encoded, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	var output, diagnostic bytes.Buffer

	args := reminderCLIArgs(t, operation, session, row)

	err = command.Run(t.Context(), client, args, &output, &diagnostic)

	checkReminderCLIOutcome(t, operation, row, output.Bytes(), err)

	if diagnostic.Len() != 0 {
		t.Fatal("reminder CLI printed unexpected diagnostics")
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func checkReminderCLIOutcome(t *testing.T, operation string, row map[string]json.RawMessage,
	output []byte, err error,
) {
	t.Helper()

	switch operation {
	case legacyRemindersCommand:
		checkLegacyReminderCLIOutcome(t, row, output, err)
	case photoStatusCommand, photoAlbumsCommand, photoCountCommand, photoAssetsCommand:
		checkPhotoCLIOutcome(t, operation, row, output, err)
	case reminderCLITagsCommand, reminderCLIAttachmentsCommand, reminderCLIRecurrenceCommand, reminderCLIAlarmsCommand:
		checkReminderCLIRelatedOutcome(t, operation, row, output, err)
	case reminderSyncCommand:
		checkReminderCLISyncOutcome(t, row, output, err)
	case reminderChangesCommand:
		checkReminderCLIChangesOutcome(t, row, output, err)
	case "reminders":
		checkReminderCLIQueryOutcome(t, row, output, err)
	case "reminder-snapshot":
		checkReminderCLISnapshotOutcome(t, row, output, err)
	default:
		checkReminderCommandOutcome(t, operation, row, output, err)
	}
}

func reminderCLIArgs(t *testing.T, operation, session string, row map[string]json.RawMessage) []string {
	t.Helper()

	args := []string{sessionFlag, session}

	if (operation == photoCountCommand || operation == photoAssetsCommand) && len(row["album"]) != 0 {
		var album string

		decode(t, row["album"], &album)
		args = append(args, "--album", album)
	}

	if operation == reminderCommand {
		var inputs []string

		decode(t, row["inputs"], &inputs)
		args = append(args, "--reminder", inputs[0])
	}

	if operation == reminderChangesCommand {
		args = append(args, reminderCLIChangesArgs(t, row)...)
	}

	if operation == "reminders" {
		args = append(args, reminderCLIQueryArgs(t, row)...)
	}

	if operation == "reminder-snapshot" {
		var inputs []string

		decode(t, row["inputs"], &inputs)

		if len(inputs) != 0 {
			args = append(args, "--list", inputs[0])
		}
	}

	switch operation {
	case reminderCLITagsCommand, reminderCLIAttachmentsCommand, reminderCLIRecurrenceCommand, reminderCLIAlarmsCommand:
		args = append(args, reminderCLIRelatedArgs(t, row)...)
	}

	return append(args, operation)
}

func checkReminderCommandOutcome(t *testing.T, operation string, row map[string]json.RawMessage,
	output []byte, err error,
) {
	t.Helper()

	if len(row["error"]) != 0 {
		var failure *icloud.ClientError
		if len(output) != 0 || !errors.As(err, &failure) {
			t.Fatal("CLI lost reminder error or printed partial data")
		}

		checkReminderCLIFailure(t, row, failure)

		return
	}

	if err != nil {
		t.Fatal(err)
	}

	var actual map[string]json.RawMessage

	decode(t, output, &actual)

	for _, field := range []string{"metadata", "responses"} {
		if _, exists := actual[field]; exists {
			t.Fatal("CLI exposed authentication response metadata")
		}
	}

	if operation == "reminder-lists" {
		checkReminderCLILists(t, actual["lists"], row["result"])

		return
	}

	if operation == reminderCommand {
		if len(actual) != 1 {
			t.Fatal("CLI reminder result contains unexpected fields")
		}

		checkReminderCLIReminder(t, actual["reminder"], row["result"])

		return
	}

	checkReminderCLIZones(t, actual, row["result"])
}

func checkReminderCLIFailure(t *testing.T, row map[string]json.RawMessage, failure *icloud.ClientError) {
	t.Helper()

	var (
		exchanges   []replay.Exchange
		sourceError map[string]string
	)

	decode(t, row["exchanges"], &exchanges)
	decode(t, row["error"], &sourceError)

	last := exchanges[len(exchanges)-1]

	kind := expectedFailureKind(last.Response.Status)
	if last.Response.Status < 400 && !reminderCLIProviderMessage(sourceError["message"]) {
		kind = icloud.InvalidResponse
	}

	if sourceError["type"] == "LookupError" {
		kind = icloud.NotFound
	}

	if failure.Kind() != kind || failure.StatusCode() != last.Response.Status ||
		!bytes.Equal(failure.ResponseBody(), referenceResponseBody(t, last.Response.Body)) {
		t.Fatal("CLI changed reminder failure class, status or response body")
	}

	metadata := icloud.ResponseMetadata{StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders(),
		CookieScopeURL: failure.CookieScopeURL()}
	if !reflect.DeepEqual(metadata, referenceResponseMetadata(last)) {
		t.Fatal("CLI lost reminder failure metadata")
	}

	checkReminderCLIPrior(t, row, failure)
}

func reminderCLIProviderMessage(message string) bool {
	for _, prefix := range []string{"Fetch reminder lists failed", "Lookup reminder failed",
		"Lookup alarms failed", "Lookup alarm triggers failed", "Lookup attachments failed",
		"Lookup recurrence rules failed", "Lookup hashtags failed", "Iterating reminder changes failed",
		"Unable to obtain sync token", "List reminders query failed"} {
		if strings.HasPrefix(message, prefix) {
			return true
		}
	}

	return false
}

func checkReminderCLILists(t *testing.T, actual, source json.RawMessage) {
	t.Helper()

	var expected []map[string]json.RawMessage

	decode(t, source, &expected)

	for _, list := range expected {
		for before, after := range map[string]string{"badge_emblem": "badgeEmblem", "sorting_style": "sortingStyle",
			"is_group": "isGroup", "reminder_ids": "reminderIDs", reminderCLISourceRevisionField: reminderCLIRevisionField} {
			list[after] = list[before]
			delete(list, before)
		}
	}

	checkReminderCLIValue(t, actual, expected)
}

func checkReminderCLIZones(t *testing.T, actual map[string]json.RawMessage, source json.RawMessage) {
	t.Helper()

	var expected map[string]json.RawMessage

	decode(t, source, &expected)

	var zones []map[string]json.RawMessage

	decode(t, expected["zones"], &zones)

	for index, zone := range zones {
		zones[index] = projectReminderCLIZone(t, zone)
	}

	checkReminderCLIValue(t, actual["zones"], zones)
	delete(expected, "zones")

	if len(expected) != 0 {
		checkReminderCLIValue(t, actual["additionalMetadata"], expected)
	}
}

func projectReminderCLIZone(t *testing.T, zone map[string]json.RawMessage) map[string]json.RawMessage {
	t.Helper()

	var identity map[string]json.RawMessage

	decode(t, zone["zoneID"], &identity)

	result := map[string]json.RawMessage{"name": identity["zoneName"], "owner": identity["ownerRecordName"],
		"type": identity["zoneType"], "syncToken": zone["syncToken"], "deleted": zone["deleted"]}

	for _, name := range []string{"zoneName", "ownerRecordName", "zoneType"} {
		delete(identity, name)
	}

	for _, name := range []string{"zoneID", "syncToken", "deleted"} {
		delete(zone, name)
	}

	if len(identity) != 0 {
		result["identityMetadata"] = reminderCLIEncode(t, identity)
	}

	if len(zone) != 0 {
		result["additionalMetadata"] = reminderCLIEncode(t, zone)
	}

	return result
}

func reminderCLIEncode(t *testing.T, value any) json.RawMessage {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}

	return encoded
}

func checkReminderCLIValue(t *testing.T, actual json.RawMessage, expected any) {
	t.Helper()

	var actualValue, expectedValue any

	actualDecoder := json.NewDecoder(bytes.NewReader(actual))
	actualDecoder.UseNumber()

	err := actualDecoder.Decode(&actualValue)
	if err != nil {
		t.Fatal(err)
	}

	expectedDecoder := json.NewDecoder(bytes.NewReader(reminderCLIEncode(t, expected)))
	expectedDecoder.UseNumber()

	err = expectedDecoder.Decode(&expectedValue)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(actualValue, expectedValue) {
		t.Fatal("complete CLI reminder result differs from Source")
	}
}
