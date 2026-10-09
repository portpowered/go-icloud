package replay_test

import (
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestFindMySavedTokenRecoverySDK(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/session-findmy-autorefresh-*.json")
	if err != nil || len(paths) != 5 {
		t.Fatal("Find My saved-token recovery inventory changed", err)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runFindMyTokenRecoverySDK(t, path)
		})
	}
}

func runFindMyTokenRecoverySDK(t *testing.T, path string) {
	t.Helper()
	row := authReplayObject(t, path)
	scenario := readAccountScenario(t, path)

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	auth := sdkAccountAuth(scenario.Initial)
	auth.FindMyServiceURL = scenario.Initial.Origin

	first, firstErr := client.OpenFindMySession(t.Context(), icloud.OpenFindMySessionRequest{Auth: auth,
		IncludeFamily: false}, icloud.WithFindMyMonitorInterval(0))
	if first != nil {
		t.Fatal("expired Find My session returned a partial session")
	}

	checkFindMyRecoveryFailure(t, firstErr, icloud.AuthenticationRequired, scenario.Exchanges[0], nil)

	inputRow := make(map[string]json.RawMessage, len(row)+1)
	maps.Copy(inputRow, row)

	inputRow["keyword_inputs"] = json.RawMessage(`{"force_refresh":true}`)

	request := authReplayRequest(t, inputRow)

	request.ResponseUpdates = []icloud.ResponseMetadata{publicFindMyFailure(t, firstErr)}
	before := marshalFindMyRecovery(t, request)

	refreshed, refreshErr := client.ResumeSession(t.Context(), request)

	if string(before) != string(marshalFindMyRecovery(t, request)) {
		t.Fatal("recovery mutated caller-owned credentials or prior-response metadata")
	}

	if len(scenario.Exchanges) == 2 {
		if refreshed != nil {
			t.Fatal("rejected token returned a partial authentication result")
		}

		checkFindMyRecoveryFailure(t, refreshErr, icloud.Unauthorized, scenario.Exchanges[1], scenario.Exchanges[:1])
	} else {
		checkFindMyRecoveredRead(t, client, row, scenario, refreshed, refreshErr)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func checkFindMyRecoveredRead(t *testing.T, client icloud.Client, row map[string]json.RawMessage,
	scenario accountScenario, refreshed *icloud.ResumeSessionResult, err error,
) {
	t.Helper()

	if err != nil || refreshed == nil {
		t.Fatal("Find My token refresh failed", err)
	}

	expected := make(map[string]json.RawMessage, len(row))
	maps.Copy(expected, row)

	if len(row["error"]) != 0 {
		expected["result"] = append(append(json.RawMessage(`{"auth_state":`), row["error_auth_state"]...), '}')
	}

	if len(row["refresh_auth_state"]) != 0 {
		expected["result"] = append(append(json.RawMessage(`{"auth_state":`), row["refresh_auth_state"]...), '}')
	}

	expected["exchanges"] = marshalFindMyRecovery(t, scenario.Exchanges[:2])
	assertResumedAuth(t, expected, refreshed)
	checkFindMyRecoveryResponses(t, refreshed.Responses, scenario.Exchanges[:2])

	// Source binds the manager to its original service origin across authentication discovery.
	readAuth := refreshed.Auth
	readAuth.FindMyServiceURL = scenario.Initial.Origin

	session, readErr := client.OpenFindMySession(t.Context(), icloud.OpenFindMySessionRequest{Auth: readAuth,
		IncludeFamily: false}, icloud.WithFindMyMonitorInterval(0))
	if len(row["error"]) != 0 {
		if session != nil {
			t.Fatal("repeated authentication refusal returned a partial session")
		}

		checkFindMyRecoveryFailure(t, readErr, icloud.AuthenticationRequired, scenario.Exchanges[2], nil)

		return
	}

	checkFindMyRecoveredSnapshot(t, session, readErr, row, scenario.Exchanges[2:])
}

func checkFindMyRecoveredSnapshot(t *testing.T, session *icloud.FindMySession, readErr error,
	row map[string]json.RawMessage, exchanges []replay.Exchange,
) {
	t.Helper()

	if readErr != nil || session == nil {
		t.Fatal("refreshed Find My read failed", readErr)
	}

	snapshot, snapshotErr := session.Snapshot()
	metadata := session.LastResponses()

	closeErr := session.Close()
	if snapshotErr != nil || closeErr != nil {
		t.Fatal(snapshotErr, closeErr)
	}

	var source map[string]json.RawMessage

	authReplayDecode(t, row["result"], &source)
	checkSDKValue(t, snapshot.Devices, source["devices"])
	checkFindMyRecoveryResponses(t, metadata, exchanges)
}

func publicFindMyFailure(t *testing.T, err error) icloud.ResponseMetadata {
	t.Helper()

	var failure *icloud.ClientError

	if !errors.As(err, &failure) {
		t.Fatal("Find My recovery lost typed provider error", err)
	}

	return icloud.ResponseMetadata{StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders(),
		CookieScopeURL: failure.CookieScopeURL()}
}

func checkFindMyRecoveryFailure(t *testing.T, err error, kind icloud.ErrorKind,
	exchange replay.Exchange, prior []replay.Exchange,
) {
	t.Helper()

	var failure *icloud.ClientError

	if !errors.As(err, &failure) || failure.Kind() != kind ||
		string(failure.ResponseBody()) != string(contractAuthBody(t, exchange.Response.Body)) {
		t.Fatal("Find My recovery lost failure classification or complete body", err)
	}

	checkFindMyRecoveryResponses(t, []icloud.ResponseMetadata{publicFindMyFailure(t, err)}, []replay.Exchange{exchange})
	checkFindMyRecoveryResponses(t, failure.PriorResponses(), prior)
}

func checkFindMyRecoveryResponses(t *testing.T, metadata []icloud.ResponseMetadata, exchanges []replay.Exchange) {
	t.Helper()

	if len(metadata) != len(exchanges) {
		t.Fatal("Find My recovery response inventory changed")
	}

	for index, actual := range metadata {
		exchange := exchanges[index]

		headers := http.Header{}

		for _, pair := range exchange.Response.Headers {
			headers.Add(pair[0], pair[1])
		}

		keys := make([]string, 0, len(headers))
		for key := range headers {
			keys = append(keys, key)
		}

		sort.Strings(keys)

		expected := []icloud.Header{}

		for _, key := range keys {
			for _, value := range headers[key] {
				expected = append(expected, icloud.Header{Name: key, Value: value})
			}
		}

		// Go's public metadata canonicalizes HTTP names; every value and repeated-header order remains bound.
		if actual.StatusCode != exchange.Response.Status || !reflect.DeepEqual(actual.Headers, expected) ||
			actual.CookieScopeURL != exchange.Request.Origin+exchange.Request.Path {
			t.Fatal("Find My recovery lost complete response metadata or cookie scope")
		}
	}
}

func marshalFindMyRecovery(t *testing.T, value any) json.RawMessage {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}

	return data
}
