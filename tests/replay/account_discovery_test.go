package replay_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestResumeThenReadDiscoveredAccount(t *testing.T) {
	t.Parallel()
	corpus := authReplayObject(t, "fixtures/synthetic/local/reference-resume.json")
	local := authReplayObject(t, "fixtures/synthetic/local/reference-logins.json")

	var cases, logins []map[string]json.RawMessage

	authReplayDecode(t, corpus["cases"], &cases)
	authReplayDecode(t, local["cases"], &logins)

	if len(cases) != 4 {
		t.Fatal("reference account flow inventory changed")
	}

	for _, row := range cases {
		var name string

		authReplayDecode(t, row["name"], &name)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runDiscoveredAccountFlow(t, row, logins)
		})
	}
}

func discoveredAccountCredentials(t *testing.T, row map[string]json.RawMessage,
	logins []map[string]json.RawMessage,
) icloud.ResumeSessionRequest {
	t.Helper()

	var expected string

	authReplayDecode(t, row["localCase"], &expected)

	for _, login := range logins {
		var filename string

		authReplayDecode(t, login["filename"], &filename)

		if filename != expected {
			continue
		}

		var (
			request          icloud.ResumeSessionRequest
			session, headers map[string]string
			identity         map[string]json.RawMessage
		)

		authReplayDecode(t, login["session"], &session)
		authReplayDecode(t, login["identity"], &identity)
		authReplayDecode(t, login["expectedHeaders"], &headers)
		authReplayDecode(t, login["expectedCookies"], &request.Auth.Cookies)
		authReplayDecode(t, login["setupOrigin"], &request.Auth.SetupServiceURL)
		authReplayDecode(t, identity["china"], &request.Auth.ChinaMainland)
		request.Auth.ClientID = session["client_id"]
		request.Auth.SessionToken = session["session_token"]
		request.TrustToken = session["trust_token"]
		request.AccountCountryCode.Set(session["account_country"])

		for name, value := range headers {
			request.Auth.Headers = append(request.Auth.Headers, icloud.Header{Name: name, Value: value})
		}

		return request
	}

	var missing icloud.ResumeSessionRequest

	t.Fatal("reference login not found")

	return missing
}

func runDiscoveredAccountFlow(t *testing.T, row map[string]json.RawMessage,
	logins []map[string]json.RawMessage,
) {
	t.Helper()

	var exchanges []replay.Exchange

	authReplayDecode(t, row["exchanges"], &exchanges)

	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	resumed, err := client.ResumeSession(t.Context(), discoveredAccountCredentials(t, row, logins))
	if err != nil {
		t.Fatal(err)
	}

	var state map[string]json.RawMessage

	authReplayDecode(t, row["authState"], &state)
	assertAuthDiscovery(t, state, resumed)

	result, err := client.GetAccountDevices(t.Context(), icloud.GetAccountDevicesRequest{Auth: resumed.Auth})
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := json.Marshal(result.Devices)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(accountJSON(t, encoded), accountJSON(t, row["devices"])) {
		t.Fatal("resumed account device projection changed")
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}
