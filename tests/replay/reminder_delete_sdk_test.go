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

func TestDeleteReminderSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"reminders-delete-success.json", "reminders-delete-record-error.json"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runDeleteReminder(t, "fixtures/synthetic/http/"+name)
		})
	}
}

func runDeleteReminder(t *testing.T, path string) {
	t.Helper()
	row := authReplayObject(t, path)
	scenario := readAccountScenario(t, path)

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	var entropy struct {
		UUIDs []string `json:"uuid4"`
		//nolint:tagliatelle // LIB-05: reference fixture spelling.
		Seconds int64 `json:"unix_seconds"`
	}

	authReplayDecode(t, row["entropy"], &entropy)

	var random bytes.Buffer

	for _, identity := range entropy.UUIDs {
		data, decodeErr := hex.DecodeString(strings.ReplaceAll(identity, "-", ""))
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}

		random.Write(data)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport), icloud.WithRandomSource(&random),
		icloud.WithClock(func() time.Time { return time.Unix(entropy.Seconds, 0) }))
	if err != nil {
		t.Fatal(err)
	}

	request := deleteReminderFixtureRequest(t, row, scenario)
	before := marshalFindMyRecovery(t, request)
	result, callErr := client.DeleteReminder(t.Context(), request)

	checkDeleteReminderOutcome(t, row, scenario, result, callErr)

	if !bytes.Equal(before, marshalFindMyRecovery(t, request)) {
		t.Fatal("delete mutated caller credentials or revision")
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}

	if random.Len() != 0 {
		t.Fatal("delete did not consume the declared resolution identities")
	}
}

func deleteReminderFixtureRequest(t *testing.T, row map[string]json.RawMessage,
	scenario accountScenario,
) icloud.DeleteReminderRequest {
	t.Helper()

	var inputs []map[string]json.RawMessage

	authReplayDecode(t, row["inputs"], &inputs)

	var values map[string]json.RawMessage

	authReplayDecode(t, inputs[0]["value"], &values)

	request := icloud.DeleteReminderRequest{Auth: sdkAccountAuth(scenario.Initial), ReminderID: "", RecordChangeTag: nil}
	request.Auth.RemindersServiceURL = scenario.Initial.Origin
	authReplayDecode(t, values["id"], &request.ReminderID)
	authReplayDecode(t, values[reminderSourceRevisionField], &request.RecordChangeTag)

	return request
}

func checkDeleteReminderOutcome(t *testing.T, row map[string]json.RawMessage, scenario accountScenario,
	result *icloud.DeleteReminderResult, callErr error,
) {
	t.Helper()

	if len(scenario.Error) != 0 {
		checkDeleteReminderFailure(t, scenario, result, callErr)

		return
	}

	if callErr != nil || result == nil {
		t.Fatal(callErr)
	}

	var expected map[string]json.RawMessage

	authReplayDecode(t, row["result"], &expected)

	var arguments []map[string]json.RawMessage

	authReplayDecode(t, expected["arguments"], &arguments)
	checkSDKValue(t, result.Deleted, arguments[0]["deleted"])
	checkSDKValue(t, result.RecordChangeTag, arguments[0][reminderSourceRevisionField])
	checkSDKValue(t, result.Modified, arguments[0]["modified"])

	if len(result.Responses) != 1 {
		t.Fatal("missing response evidence")
	}

	checkSDKMetadata(t, result.Responses[0], scenario.Exchanges[0].Response)
}

func checkDeleteReminderFailure(t *testing.T, scenario accountScenario, result *icloud.DeleteReminderResult,
	callErr error,
) {
	t.Helper()

	var failure *icloud.ClientError
	if result != nil || !errors.As(callErr, &failure) || failure.Kind() != icloud.Provider {
		t.Fatal("record rejection returned a success or wrong failure", callErr)
	}

	if failure.StatusCode() != scenario.Exchanges[0].Response.Status ||
		!bytes.Equal(failure.ResponseBody(), contractAuthBody(t, scenario.Exchanges[0].Response.Body)) {
		t.Fatal("record rejection lost its status or exact provider body")
	}

	checkSDKMetadata(t, icloud.ResponseMetadata{StatusCode: failure.StatusCode(),
		Headers: failure.ResponseHeaders(), CookieScopeURL: failure.CookieScopeURL()}, scenario.Exchanges[0].Response)
}
