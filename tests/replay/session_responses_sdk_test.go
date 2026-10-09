package replay_test

import (
	"encoding/json"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestApplyFailedFindMyRetryResponses(t *testing.T) {
	t.Parallel()

	path := "fixtures/synthetic/http/session-findmy-autorefresh-failed-retry-rotation.json"
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

	native := resumeForFailedFindMyUpdate(t, client, row, scenario)
	auth := native.Auth
	auth.FindMyServiceURL = scenario.Initial.Origin

	session, readErr := client.OpenFindMySession(t.Context(), icloud.OpenFindMySessionRequest{Auth: auth,
		IncludeFamily: false},
		icloud.WithFindMyMonitorInterval(0))
	if session != nil {
		t.Fatal("failed retry returned partial session")
	}

	checkFindMyRecoveryFailure(t, readErr, icloud.AuthenticationRequired, scenario.Exchanges[2], nil)
	request := icloud.ApplySessionResponsesRequest{Session: *native,
		Responses: []icloud.ResponseMetadata{publicFindMyFailure(t, readErr)}}
	before := marshalFindMyRecovery(t, request)

	updated, err := client.ApplySessionResponses(t.Context(), request)
	if err != nil || updated == nil {
		t.Fatal("applying failed response metadata failed", err)
	}

	if string(before) != string(marshalFindMyRecovery(t, request)) {
		t.Fatal("response application mutated caller state")
	}

	checkAppliedFailedFindMyState(t, row, scenario, updated.Session)
	updated.Session.Auth.Cookies[0].Value = "mutated-result"
	updated.Session.AccountData[0] = ' '
	updated.Session.Responses[0].Headers[0].Value = "mutated-result"

	if string(before) != string(marshalFindMyRecovery(t, request)) {
		t.Fatal("response result aliases caller-owned data")
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func resumeForFailedFindMyUpdate(t *testing.T, client icloud.Client, row map[string]json.RawMessage,
	scenario accountScenario,
) *icloud.ResumeSessionResult {
	t.Helper()

	auth := sdkAccountAuth(scenario.Initial)
	auth.FindMyServiceURL = scenario.Initial.Origin

	first, err := client.OpenFindMySession(t.Context(), icloud.OpenFindMySessionRequest{Auth: auth, IncludeFamily: false},
		icloud.WithFindMyMonitorInterval(0))
	if first != nil {
		t.Fatal("expired session returned partial data")
	}

	checkFindMyRecoveryFailure(t, err, icloud.AuthenticationRequired, scenario.Exchanges[0], nil)

	input := maps.Clone(row)
	input["keyword_inputs"] = json.RawMessage(`{"force_refresh":true}`)
	request := authReplayRequest(t, input)
	request.ForceRefresh = true
	request.ResponseUpdates = []icloud.ResponseMetadata{publicFindMyFailure(t, err)}

	result, resumeErr := client.ResumeSession(t.Context(), request)
	if resumeErr != nil || result == nil {
		t.Fatal(resumeErr)
	}

	expected := maps.Clone(row)
	expected["result"] = append(append(json.RawMessage(`{"auth_state":`), row["refresh_auth_state"]...), '}')
	expected["exchanges"] = marshalFindMyRecovery(t, scenario.Exchanges[:2])
	assertResumedAuth(t, expected, result)

	return result
}

func checkAppliedFailedFindMyState(t *testing.T, row map[string]json.RawMessage,
	scenario accountScenario, result icloud.ResumeSessionResult,
) {
	t.Helper()

	expected := maps.Clone(row)
	expected["result"] = append(append(json.RawMessage(`{"auth_state":`), row["error_auth_state"]...), '}')
	assertResumedAuth(t, expected, &result)
	checkFindMyRecoveryResponses(t, result.Responses, scenario.Exchanges)

	var state map[string]json.RawMessage

	authReplayDecode(t, row["error_auth_state"], &state)
	checkAppliedFindMyCookies(t, result.Auth.Cookies, state["cookies"])
}

func checkAppliedFindMyCookies(t *testing.T, actual []icloud.AuthCookie, raw json.RawMessage) {
	t.Helper()

	var expected []map[string]json.RawMessage

	authReplayDecode(t, raw, &expected)

	if len(actual) != len(expected) {
		t.Fatal("updated cookie inventory differs from Source")
	}

	for index, record := range expected {
		var cookie icloud.AuthCookie

		authReplayDecode(t, marshalFindMyRecovery(t, record), &cookie)

		var attributes map[string]json.RawMessage

		authReplayDecode(t, record["attributes"], &attributes)
		_, cookie.HTTPOnly = attributes["HttpOnly"]
		cookie.HostOnly = !strings.HasPrefix(cookie.Domain, ".")

		var expires *int64

		authReplayDecode(t, record["expires"], &expires)

		if expires != nil {
			instant := time.Unix(*expires, 0).UTC()
			cookie.Expires = &instant
		}

		got := actual[index]
		got.Domain = strings.TrimPrefix(got.Domain, ".")

		cookie.Domain = strings.TrimPrefix(cookie.Domain, ".")

		if string(marshalFindMyRecovery(t, got)) != string(marshalFindMyRecovery(t, cookie)) {
			t.Fatal("updated cookie attributes differ from Source")
		}
	}
}
