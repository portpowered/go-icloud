package replay_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const findMyCommand = "findmy"

func TestFindMyNativeTokenRecoveryCommand(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("../../../../tests/replay/fixtures/synthetic/http/session-findmy-autorefresh-*.json")
	if err != nil || len(paths) != 6 {
		t.Fatal("Find My recovery CLI inventory changed", err)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) { t.Parallel(); runFindMyRecoveryCommand(t, readObject(t, path)) })
	}
}

func runFindMyRecoveryCommand(t *testing.T, row map[string]json.RawMessage) {
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

	initial := findMyRecoveryNativeInput(t, row)

	encoded, err := json.Marshal(initial)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "session.json")

	err = os.WriteFile(path, encoded, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	var output, diagnostic bytes.Buffer

	err = command.Run(t.Context(), client, []string{sessionFlag, path, findMyCommand}, &output, &diagnostic)

	if diagnostic.Len() != 0 {
		t.Fatal("Find My recovery printed diagnostics")
	}

	checkFindMyRecoveryCLIOutcome(t, row, output.Bytes(), err, exchanges)

	saved, readErr := os.ReadFile(filepath.Clean(path))
	if readErr != nil {
		t.Fatal(readErr)
	}

	if len(exchanges) == 2 {
		if !bytes.Equal(saved, encoded) {
			t.Fatal("rejected refresh changed saved credentials")
		}
	} else {
		checkFindMyRecoveredNativeState(t, row, saved, exchanges)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func findMyRecoveryNativeInput(t *testing.T, row map[string]json.RawMessage) icloud.ResumeSessionResult {
	t.Helper()
	initial := nativeAuthInput(t, row)

	var (
		state  map[string]json.RawMessage
		params map[string]string
	)

	decode(t, row["initial_state"], &state)
	decode(t, state["params"], &params)
	initial.Auth.AccountID = params["dsid"]
	build, mastering := params["clientBuildNumber"], params["clientMasteringNumber"]
	initial.Auth.ClientBuildNumber = &build
	initial.Auth.ClientMasteringNumber = &mastering
	decode(t, state["origin"], &initial.Auth.FindMyServiceURL)

	return initial
}

func checkFindMyRecoveryCLIOutcome(t *testing.T, row map[string]json.RawMessage, output []byte,
	err error, exchanges []replay.Exchange,
) {
	t.Helper()

	if len(row["error"]) == 0 {
		if err != nil {
			t.Fatal(err)
		}

		var result map[string]json.RawMessage

		decode(t, row["result"], &result)
		checkReminderCLIValue(t, output, result["devices"])

		return
	}

	checkFindMyRecoveryCLIFailure(t, output, err, exchanges)
}

func checkFindMyRecoveryCLIFailure(t *testing.T, output []byte, err error, exchanges []replay.Exchange) {
	t.Helper()

	var failure *icloud.ClientError
	if len(output) != 0 || !errors.As(err, &failure) {
		t.Fatal("Find My recovery lost typed error or printed partial result")
	}

	last := exchanges[len(exchanges)-1]

	kind := icloud.AuthenticationRequired
	if last.Response.Status == 401 {
		kind = icloud.Unauthorized
	}

	metadata := icloud.ResponseMetadata{StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders(),
		CookieScopeURL: failure.CookieScopeURL()}
	if failure.Kind() != kind || !bytes.Equal(failure.ResponseBody(), referenceResponseBody(t, last.Response.Body)) ||
		!reflect.DeepEqual(metadata, referenceResponseMetadata(last)) {
		t.Fatal("Find My recovery lost failure kind or complete response evidence")
	}

	checkFindMyRecoveryCLIPrior(t, failure, exchanges)
}

func checkFindMyRecoveryCLIPrior(t *testing.T, failure *icloud.ClientError, exchanges []replay.Exchange) {
	t.Helper()

	prior := []icloud.ResponseMetadata{}
	if len(exchanges) == 2 {
		prior = append(prior, referenceResponseMetadata(exchanges[0]))
	}

	actual := failure.PriorResponses()
	if len(actual) != len(prior) {
		t.Fatal("Find My recovery lost prior response count")
	}

	for index := range actual {
		if !reflect.DeepEqual(actual[index], prior[index]) {
			t.Fatal("Find My recovery changed prior response evidence")
		}
	}
}

func checkFindMyRecoveredNativeState(t *testing.T, row map[string]json.RawMessage, saved []byte,
	exchanges []replay.Exchange,
) {
	t.Helper()

	var actual icloud.ResumeSessionResult

	decode(t, saved, &actual)

	var initial, state, result map[string]json.RawMessage

	decode(t, row["initial_state"], &initial)
	stateRaw := row["error_auth_state"]

	if len(row["error"]) == 0 {
		decode(t, row["result"], &result)
		stateRaw = result["auth_state"]
	}

	decode(t, stateRaw, &state)
	expectedRow := map[string]json.RawMessage{"authState": stateRaw, "headers": initial["headers"],
		"cookieState": nativeSourceCookieState(t, state)}
	expected := expectedReferenceSession(t, expectedRow, "authState", exchanges[1])

	expected.Responses = []icloud.ResponseMetadata{}
	for _, exchange := range exchanges {
		expected.Responses = append(expected.Responses, referenceResponseMetadata(exchange))
	}

	assertSavedSessionSnapshot(t, actual, expected)
}
