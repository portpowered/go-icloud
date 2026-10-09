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

func TestReminderCommands(t *testing.T) {
	t.Parallel()

	for prefix, operation := range map[string]string{"zones": "reminder-zones",
		"lists": "reminder-lists", "get": reminderCommand} {
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

	args := []string{sessionFlag, session}

	if operation == reminderCommand {
		var inputs []string

		decode(t, row["inputs"], &inputs)
		args = append(args, "--reminder", inputs[0])
	}

	args = append(args, operation)
	err = command.Run(t.Context(), client, args, &output, &diagnostic)
	checkReminderCommandOutcome(t, operation, row, output.Bytes(), err)

	if diagnostic.Len() != 0 {
		t.Fatal("reminder CLI printed unexpected diagnostics")
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
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
	if last.Response.Status < 400 && !strings.HasPrefix(sourceError["message"], "Fetch reminder lists failed") &&
		!strings.HasPrefix(sourceError["message"], "Lookup reminder failed") {
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
}

func checkReminderCLILists(t *testing.T, actual, source json.RawMessage) {
	t.Helper()

	var expected []map[string]json.RawMessage

	decode(t, source, &expected)

	for _, list := range expected {
		for before, after := range map[string]string{"badge_emblem": "badgeEmblem", "sorting_style": "sortingStyle",
			"is_group": "isGroup", "reminder_ids": "reminderIDs", "record_change_tag": "recordChangeTag"} {
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
