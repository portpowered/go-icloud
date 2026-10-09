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

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestNativeResumeCommands(t *testing.T) {
	t.Parallel()

	names := []string{"auth-authenticate-cloudkit-discovery", "auth-authenticate-cached", "auth-authenticate-paused",
		"auth-authenticate-refresh",
		"auth-authenticate-untrusted-refresh", "auth-authenticate-stale-token", "auth-token-cookie-rotation",
		"auth-authenticate-validation-201", "auth-authenticate-refresh-202", "auth-authenticate-empty-headers",
		"auth-authenticate-empty-headers-refresh", "auth-authenticate-quoted-cookie",
		"auth-authenticate-quoted-cookie-rotation", "auth-authenticate-explicit-cookie",
		"auth-terms-refused", "auth-token-login-needs-2fa", "auth-authenticate-untrusted-no-password"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) { t.Parallel(); runNativeResume(t, name) })
	}
}

func runNativeResume(t *testing.T, name string) {
	t.Helper()
	raw := readObject(t, filepath.Join("../../../../tests/replay/fixtures/synthetic/http", name+".json"))

	var exchanges []replay.Exchange

	decode(t, raw["exchanges"], &exchanges)

	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	initial := nativeAuthInput(t, raw)

	encoded, err := json.Marshal(initial)
	if err != nil {
		t.Fatal(err)
	}

	source := filepath.Join(t.TempDir(), "initial.json")
	destination := filepath.Join(t.TempDir(), "saved.json")

	err = os.WriteFile(source, encoded, 0600)
	if err != nil {
		t.Fatal(err)
	}

	args := nativeResumeArgs(t, raw, source, destination)

	var output, diagnostic bytes.Buffer

	err = command.Run(t.Context(), client, args, &output, &diagnostic)
	assertResumeCommandOutcome(t, name, raw, err, output.Bytes(), destination)
	assertResumeInputUnchanged(t, source, encoded, diagnostic.Bytes())

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func assertResumeCommandOutcome(t *testing.T, name string, raw map[string]json.RawMessage,
	err error, output []byte, destination string,
) {
	t.Helper()

	if _, failed := raw["error"]; failed {
		assertResumeRefusal(t, name, raw, err, output, destination)

		return
	}

	if err != nil {
		t.Fatal(err)
	}

	assertNativeSaved(t, raw, destination, output)
}

func assertResumeInputUnchanged(t *testing.T, path string, encoded, diagnostic []byte) {
	t.Helper()

	after, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(after, encoded) || len(diagnostic) != 0 {
		t.Fatal("resume changed its input or printed diagnostics")
	}
}

func nativeAuthInput(t *testing.T, raw map[string]json.RawMessage) icloud.ResumeSessionResult {
	t.Helper()

	var initial map[string]json.RawMessage

	decode(t, raw["initial_state"], &initial)

	var params, session, headers map[string]string

	decode(t, initial["params"], &params)
	decode(t, initial["session_data"], &session)
	decode(t, initial["headers"], &headers)

	var result icloud.ResumeSessionResult

	result.Auth.ClientID = params["clientId"]
	result.Auth.SetupServiceURL = "https://setup.icloud.com"
	token := session["session_token"]
	result.Auth.SessionToken = &token

	result.TrustToken = session["trust_token"]

	if country, exists := session["account_country"]; exists {
		result.AccountCountryCode.Set(country)
	}

	decode(t, initial["cookies"], &result.Auth.Cookies)

	for index := range result.Auth.Cookies {
		result.Auth.Cookies[index].HTTPOnly = true
	}

	for name, value := range headers {
		result.Auth.Headers = append(result.Auth.Headers, icloud.Header{Name: name, Value: value})
	}

	return result
}

func nativeResumeArgs(t *testing.T, raw map[string]json.RawMessage, source, destination string) []string {
	t.Helper()

	args := []string{sessionFlag, source, saveSessionFlag, destination}

	var keywords map[string]bool

	decode(t, raw["keyword_inputs"], &keywords)

	_, failed := raw["error"]
	if keywords["force_refresh"] || failed {
		args = append(args, "--force-refresh")
	}

	if keywords["pause_2fa"] {
		args = append(args, "--allow-untrusted")
	}

	return append(args, resumeCommand)
}

func assertNativeSaved(t *testing.T, raw map[string]json.RawMessage, path string, output []byte) {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	var result icloud.ResumeSessionResult

	decode(t, data, &result)
	assertCompleteNativeSession(t, raw, result)

	var expected, state map[string]json.RawMessage

	decode(t, raw["result"], &expected)
	decode(t, expected["auth_state"], &state)

	var actualAccount, expectedAccount any

	decode(t, result.AccountData, &actualAccount)
	decode(t, state["account"], &expectedAccount)

	if !reflect.DeepEqual(actualAccount, expectedAccount) {
		t.Fatal("private saved session changed discovery")
	}

	var session map[string]string

	decode(t, state["session_data"], &session)

	if result.Auth.SessionToken == nil || *result.Auth.SessionToken != session["session_token"] ||
		result.TrustToken != session["trust_token"] {
		t.Fatal("private saved credentials changed")
	}

	var summary map[string]json.RawMessage

	decode(t, output, &summary)

	if len(summary) != 4 {
		t.Fatal("resume printed additional state")
	}

	var savedPath string

	decode(t, summary["sessionFile"], &savedPath)

	if savedPath != path {
		t.Fatal("resume printed wrong session destination")
	}

	assertResumeConsolePrivacy(t, session, output)

	assertResumeFlags(t, state, summary, result)
}

func assertResumeConsolePrivacy(t *testing.T, session map[string]string, output []byte) {
	t.Helper()

	secrets := []string{session["session_token"], session["trust_token"], "synthetic-cookie", "synthetic-dsid"}
	for _, secret := range secrets {
		if secret != "" && strings.Contains(string(output), secret) {
			t.Fatal("resume printed private credentials")
		}
	}
}

func assertResumeFlags(t *testing.T, state, summary map[string]json.RawMessage, result icloud.ResumeSessionResult) {
	t.Helper()

	fields := map[string]string{
		"trusted": "trusted_session", "requiresTwoFactor": "requires_2fa", "requiresTwoStep": "requires_2sa",
	}
	for key, source := range fields {
		var expected, actual bool

		decode(t, state[source], &expected)
		decode(t, summary[key], &actual)

		if actual != expected {
			t.Fatal("resume changed Source authentication flags")
		}
	}

	var trusted bool

	decode(t, summary["trusted"], &trusted)

	if trusted != result.TrustedSession {
		t.Fatal("saved trust differs from displayed trust")
	}
}

func assertResumeRefusal(t *testing.T, name string, raw map[string]json.RawMessage,
	err error, output []byte, path string,
) {
	t.Helper()

	var failure *icloud.ClientError
	if !errors.As(err, &failure) || len(output) != 0 {
		t.Fatal("resume lost typed refusal or printed credentials")
	}

	expectedKind := icloud.AuthenticationRequired
	if name == "auth-terms-refused" {
		expectedKind = icloud.TermsRequired
	}

	if failure.Kind() != expectedKind {
		t.Fatal("resume changed refusal class")
	}

	var exchanges []replay.Exchange

	decode(t, raw["exchanges"], &exchanges)
	response := exchanges[len(exchanges)-1].Response

	var encoded string

	decode(t, response.Body.Value, &encoded)

	body, bodyErr := base64.StdEncoding.DecodeString(encoded)
	if bodyErr != nil {
		t.Fatal(bodyErr)
	}

	if failure.StatusCode() != response.Status || !bytes.Equal(failure.ResponseBody(), body) {
		t.Fatal("resume changed refusal status")
	}

	_, statErr := os.Stat(path)
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("refusal replaced private saved state")
	}
}
