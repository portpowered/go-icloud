package replay_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestResumeSavedSessions(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"auth-authenticate-cloudkit-discovery",
		"auth-authenticate-cached", "auth-authenticate-paused", "auth-authenticate-refresh",
		"auth-authenticate-untrusted-refresh", "auth-authenticate-stale-token", "auth-token-cookie-rotation",
		"auth-authenticate-validation-201", "auth-authenticate-refresh-202",
		"auth-authenticate-empty-headers", "auth-authenticate-empty-headers-refresh",
		"auth-authenticate-quoted-cookie", "auth-authenticate-quoted-cookie-rotation", "auth-authenticate-explicit-cookie"} {
		t.Run(name, func(t *testing.T) { t.Parallel(); resumeSavedSession(t, name) })
	}
}

func resumeSavedSession(t *testing.T, name string) {
	t.Helper()
	raw := authReplayObject(t, filepath.Join("fixtures/synthetic/http", name+".json"))

	var exchanges []replay.Exchange

	authReplayDecode(t, raw["exchanges"], &exchanges)

	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	request := authReplayRequest(t, raw)

	before, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.ResumeSession(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}

	assertResumedAuth(t, raw, result)

	after, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}

	if string(before) != string(after) {
		t.Fatal("session resumption mutated caller credentials")
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func authReplayRequest(t *testing.T, raw map[string]json.RawMessage) icloud.ResumeSessionRequest {
	t.Helper()

	var initial map[string]json.RawMessage

	authReplayDecode(t, raw["initial_state"], &initial)

	var params, session, headers map[string]string

	authReplayDecode(t, initial["params"], &params)
	authReplayDecode(t, initial["session_data"], &session)
	authReplayDecode(t, initial["headers"], &headers)

	var cookies []referenceCookie

	authReplayDecode(t, initial["cookies"], &cookies)

	var request icloud.ResumeSessionRequest

	request.Auth.ClientID = params[protocol.ClientIDName]
	request.Auth.SetupServiceURL = protocol.AuthAccountServer0
	request.Auth.SessionToken = session["session_token"]

	request.Auth.Cookies = sdkAccountCookies(cookies)
	for index := range request.Auth.Cookies {
		request.Auth.Cookies[index].HTTPOnly = true
	}

	for key, value := range headers {
		request.Auth.Headers = append(request.Auth.Headers, icloud.Header{Name: key, Value: value})
	}

	request.TrustToken = session["trust_token"]
	if country, exists := session["account_country"]; exists {
		request.AccountCountryCode.Set(country)
	}

	var keywords map[string]bool

	authReplayDecode(t, raw["keyword_inputs"], &keywords)
	request.ForceRefresh = keywords["force_refresh"]
	request.AllowUntrusted = keywords["pause_2fa"]

	return request
}

func assertResumedAuth(t *testing.T, raw map[string]json.RawMessage, result *icloud.ResumeSessionResult) {
	t.Helper()

	var expected, state map[string]json.RawMessage

	authReplayDecode(t, raw["result"], &expected)
	authReplayDecode(t, expected["auth_state"], &state)

	if !reflect.DeepEqual(accountJSON(t, result.AccountData), accountJSON(t, state["account"])) {
		t.Fatal("resumption changed account discovery")
	}

	assertAuthFlags(t, state, result)
	assertAuthDiscovery(t, state, result)
	assertAuthResponses(t, raw, result)

	var params, session map[string]string

	authReplayDecode(t, state["params"], &params)
	authReplayDecode(t, state["session_data"], &session)

	if result.Auth.AccountID != params[protocol.DSIDName] || result.Auth.ClientID != params[protocol.ClientIDName] ||
		result.Auth.SessionToken == nil || *result.Auth.SessionToken != session["session_token"] ||
		result.TrustToken != session["trust_token"] {
		t.Fatal("resumption lost account identity or rotated tokens")
	}

	assertResumedCountry(t, session, result)
	assertResumedCookies(t, state, result)
}

func assertResumedCountry(t *testing.T, session map[string]string, result *icloud.ResumeSessionResult) {
	t.Helper()

	if country, exists := session["account_country"]; exists &&
		(!result.AccountCountryCode.IsSpecified() || result.AccountCountryCode.IsNull() ||
			result.AccountCountryCode.MustGet() != country) {
		t.Fatal("resumption changed account country")
	}
}

func assertResumedCookies(t *testing.T, state map[string]json.RawMessage, result *icloud.ResumeSessionResult) {
	t.Helper()

	var cookies []referenceCookie

	authReplayDecode(t, state["cookies"], &cookies)

	if len(result.Auth.Cookies) != len(cookies) {
		t.Fatal("resumption lost cookies")
	}

	for index, cookie := range cookies {
		if result.Auth.Cookies[index].Name != cookie.Name || result.Auth.Cookies[index].Value != cookie.Value {
			t.Fatal("resumption lost rotated cookie value")
		}
	}
}

func assertAuthFlags(t *testing.T, state map[string]json.RawMessage, result *icloud.ResumeSessionResult) {
	t.Helper()

	var trusted, twoFactor, twoStep bool

	authReplayDecode(t, state["trusted_session"], &trusted)
	authReplayDecode(t, state["requires_2fa"], &twoFactor)
	authReplayDecode(t, state["requires_2sa"], &twoStep)

	if result.TrustedSession != trusted || result.RequiresTwoFactor != twoFactor || result.RequiresTwoStep != twoStep {
		t.Fatal("resumption changed MFA or trust state")
	}
}

func authReplayObject(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	var value map[string]json.RawMessage

	authReplayDecode(t, data, &value)

	return value
}

func authReplayDecode(t *testing.T, data []byte, result any) {
	t.Helper()

	err := json.Unmarshal(data, result)
	if err != nil {
		t.Fatal(err)
	}
}

func TestResumeAuthenticationFailures(t *testing.T) {
	t.Parallel()

	cases := map[string]icloud.ErrorKind{
		"auth-terms-refused":                      icloud.TermsRequired,
		"auth-token-login-needs-2fa":              icloud.AuthenticationRequired,
		"auth-authenticate-untrusted-no-password": icloud.AuthenticationRequired,
	}
	for name, kind := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			raw := authReplayObject(t, filepath.Join("fixtures/synthetic/http", name+".json"))

			var exchanges []replay.Exchange

			authReplayDecode(t, raw["exchanges"], &exchanges)

			transport, err := replay.NewHTTPTransport(exchanges)
			if err != nil {
				t.Fatal(err)
			}

			client, err := icloud.New(icloud.WithHTTPTransport(transport))
			if err != nil {
				t.Fatal(err)
			}

			request := authReplayRequest(t, raw)
			request.ForceRefresh = true
			result, err := client.ResumeSession(t.Context(), request)
			assertResumeFailure(t, result, err, kind, exchanges[len(exchanges)-1])

			err = transport.AssertConsumed()
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func assertResumeFailure(t *testing.T, result *icloud.ResumeSessionResult, err error,
	kind icloud.ErrorKind, exchange replay.Exchange,
) {
	t.Helper()

	var failure *icloud.ClientError
	if result != nil || !errors.As(err, &failure) || failure.Kind() != kind {
		t.Fatalf("authentication failure: %v", err)
	}

	body := contractAuthBody(t, exchange.Response.Body)
	if failure.StatusCode() != exchange.Response.Status || !bytes.Equal(failure.ResponseBody(), body) {
		t.Fatal("authentication failure lost exact response")
	}

	if strings.Contains(err.Error(), string(body)) {
		t.Fatal("authentication error exposed account data")
	}
}

func contractAuthBody(t *testing.T, entity replay.Entity) []byte {
	t.Helper()

	var encoded string

	authReplayDecode(t, entity.Value, &encoded)

	body, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}

	return body
}

func assertAuthDiscovery(t *testing.T, state map[string]json.RawMessage, result *icloud.ResumeSessionResult) {
	t.Helper()

	var services map[string]map[string]string

	authReplayDecode(t, state["webservices"], &services)

	if result.Auth.DriveServiceURL != services[protocol.AuthWebServicesDrivews][protocol.AuthServiceUrl] ||
		result.Auth.RemindersServiceURL != services[protocol.AuthWebServicesCkdatabasews][protocol.AuthServiceUrl] ||
		result.Auth.FindMyServiceURL != services[protocol.AuthWebServicesFindme][protocol.AuthServiceUrl] ||
		result.Auth.DriveDocumentServiceURL != services[protocol.AuthWebServicesDocws][protocol.AuthServiceUrl] ||
		result.Auth.AccountServiceURL != services[protocol.AuthWebServicesAccount][protocol.AuthServiceUrl] {
		t.Fatal("authentication changed service discovery")
	}

	var params map[string]string

	authReplayDecode(t, state["params"], &params)

	if result.Auth.ClientBuildNumber == nil || *result.Auth.ClientBuildNumber != params[protocol.ClientBuildNumberName] ||
		result.Auth.ClientMasteringNumber == nil ||
		*result.Auth.ClientMasteringNumber != params[protocol.ClientMasteringNumberName] {
		t.Fatal("authentication lost reference build parameters")
	}
}

// This control binds omitted cookie scope and proves it stays off another service origin.
func TestResumeCookieScopeAndCrossServiceReuse(t *testing.T) {
	t.Parallel()
	raw := authReplayObject(t, "fixtures/synthetic/http/auth-authenticate-cached.json")
	request := authReplayRequest(t, raw)
	request.Auth.Cookies[0].Domain = ""
	request.Auth.Cookies[0].Path = ""

	exchanges := make([]replay.Exchange, 0, 2)

	authReplayDecode(t, raw["exchanges"], &exchanges)
	exchanges[0].Response.Headers = append(exchanges[0].Response.Headers,
		replay.Pair{accountCookieUpdateHeader, "X-APPLE-WEBAUTH-TOKEN=synthetic-scoped; Path=/setup/ws/1; Secure"})
	drive := authReplayObject(t, "fixtures/synthetic/http/drive-apps-empty.json")

	var driveExchanges []replay.Exchange

	authReplayDecode(t, drive["exchanges"], &driveExchanges)
	driveExchanges[0].Request.Query = []replay.Pair{
		{protocol.ClientBuildNumberName, protocol.AuthClientBuildNumberValue},
		{protocol.ClientMasteringNumberName, protocol.AuthClientMasteringNumberValue},
		{protocol.ClientIDName, "synthetic-client"},
		{protocol.DSIDName, "synthetic-dsid"},
	}
	exchanges = append(exchanges, driveExchanges...)

	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.ResumeSession(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}

	assertBoundAuthCookie(t, result.Auth.Cookies)

	_, err = client.ListDriveLibraries(t.Context(), icloud.ListDriveLibrariesRequest{Auth: result.Auth})
	if err != nil {
		t.Fatal(err)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func assertBoundAuthCookie(t *testing.T, cookies []icloud.AuthCookie) {
	t.Helper()

	if len(cookies) != 1 {
		t.Fatal("stale unscoped cookie survived replacement")
	}

	cookie := cookies[0]
	if cookie.Name != protocol.AuthWebAuthCookieNameValue || cookie.Value != "synthetic-scoped" ||
		cookie.Domain != "setup.icloud.com" || cookie.Path != "/setup/ws/1" || !cookie.HostOnly || !cookie.Secure {
		t.Fatal("resumption changed rotated cookie scope")
	}
}

func assertAuthResponses(t *testing.T, raw map[string]json.RawMessage, result *icloud.ResumeSessionResult) {
	t.Helper()

	var exchanges []replay.Exchange

	authReplayDecode(t, raw["exchanges"], &exchanges)

	if len(result.Responses) != len(exchanges) {
		t.Fatal("authentication lost intermediate responses")
	}

	for index, exchange := range exchanges {
		reply := result.Responses[index]
		if reply.StatusCode != exchange.Response.Status || len(reply.Headers) != len(exchange.Response.Headers) {
			t.Fatal("authentication lost response metadata")
		}

		for _, expected := range exchange.Response.Headers {
			found := false

			for _, header := range reply.Headers {
				if strings.EqualFold(header.Name, expected[0]) && header.Value == expected[1] {
					found = true
				}
			}

			if !found {
				t.Fatal("authentication changed response headers")
			}
		}
	}
}
