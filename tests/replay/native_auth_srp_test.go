package replay_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func TestNativeSRPReplay(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"auth-srp-s2k", "auth-srp-s2k_fo", "auth-srp-trust-token",
		"auth-srp-paused-mfa", "auth-srp-sms-mfa", "auth-srp-authorize-refused", "auth-srp-init-refused",
		"auth-srp-complete-refused"} {
		t.Run(name, func(t *testing.T) { t.Parallel(); nativeSRPReplay(t, name) })
	}
}

func nativeSRPReplay(t *testing.T, name string) {
	t.Helper()
	raw, transport, state := nativeFlowFixture(t, name)
	initial := authReplayObjectBytes(t, raw["initial_state"])
	var password string
	if encoded, exists := initial["synthetic_password"]; exists {
		authReplayDecode(t, encoded, &password)
	}
	var random []byte

	if _, exists := raw["entropy"]; exists {
		random = nativeFixtureEntropy(t, raw)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport), icloud.WithRandomSource(bytes.NewReader(random)))
	if err != nil {
		t.Fatal(err)
	}
	var keywords map[string]bool
	authReplayDecode(t, raw["keyword_inputs"], &keywords)
	request := icloud.AuthenticateRequest{Auth: state.Auth, AccountName: state.AccountName, Password: password,
		TrustToken: state.TrustToken, AccountCountryCode: state.AccountCountryCode, SavedState: &state,
		ForceRefresh: keywords["force_refresh"], PauseTwoFactor: keywords["pause_2fa"], Service: nil, AcceptTerms: false}
	before, err := json.Marshal(request) //nolint:gosec // G117: synthetic-only ownership snapshot; never exported.
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Authenticate(t.Context(), request)
	nativeFlowExpectedError(t, raw, err)
	if err == nil {
		nativeAssertState(t, raw, result)
	}
	after, marshalErr := json.Marshal(request) //nolint:gosec // G117: synthetic-only ownership snapshot; never exported.
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("native SRP changed caller-owned credentials")
	}
	if err = transport.AssertConsumed(); err != nil {
		t.Fatal(err)
	}
}

func nativeAssertState(t *testing.T, raw map[string]json.RawMessage, result *icloud.NativeAuthResult) {
	t.Helper()
	expected := authReplayObjectBytes(t, raw["result"])
	state := authReplayObjectBytes(t, expected["auth_state"])
	if !reflect.DeepEqual(accountJSON(t, result.State.AccountData), accountJSON(t, state["account"])) {
		t.Fatal("native authentication changed account discovery")
	}
	var requires, requested bool
	var delivery string
	authReplayDecode(t, state["requires_mfa"], &requires)
	authReplayDecode(t, state["code_requested"], &requested)
	authReplayDecode(t, state["delivery_method"], &delivery)
	if result.State.RequiresMFA != requires || result.State.CodeRequested != requested ||
		string(result.State.DeliveryMethod) != delivery {
		t.Fatalf("native challenge status differs: %+v", result.State.Challenge)
	}
	provider := result.State.Challenge.ProviderData
	if len(provider) == 0 {
		provider = []byte("{}")
	}
	if !reflect.DeepEqual(accountJSON(t, provider), accountJSON(t, state["challenge"])) {
		t.Fatal("native authentication lost normalized challenge metadata")
	}
	if notice, exists := state["delivery_notice"]; exists {
		var expectedNotice string
		authReplayDecode(t, notice, &expectedNotice)
		if result.State.DeliveryNotice == nil || *result.State.DeliveryNotice != expectedNotice {
			t.Fatal("native authentication lost delivery notice")
		}
	}
	projection := icloud.ResumeSessionResult{Auth: result.State.Auth, TrustToken: result.State.TrustToken,
		AccountCountryCode: result.State.AccountCountryCode, AccountData: result.State.AccountData,
		Responses:      result.Responses,
		TrustedSession: result.TrustedSession, RequiresTwoFactor: result.RequiresTwoFactor,
		RequiresTwoStep: result.RequiresTwoStep}
	assertAuthFlags(t, state, &projection)
	assertResumedCookies(t, state, &projection)
	session := nativeFixtureSession(t, state)
	assertResumedCountry(t, session, &projection)
	if token, exists := session["session_token"]; exists &&
		(result.State.Auth.SessionToken == nil || *result.State.Auth.SessionToken != token) {
		t.Fatal("native authentication lost session token rotation")
	}
	if trust, exists := session["trust_token"]; exists && result.State.TrustToken != trust {
		t.Fatal("native authentication lost trust token rotation")
	}
	assertAuthDiscovery(t, state, &projection)
	nativeFlowResponses(t, raw, result.Responses)
}

func nativeFixtureSession(t *testing.T, state map[string]json.RawMessage) map[string]string {
	t.Helper()
	var session map[string]string
	authReplayDecode(t, state["session_data"], &session)
	return session
}

func TestNativeSavedSessionReplay(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"auth-authenticate-cloudkit-discovery", "auth-authenticate-cached",
		"auth-authenticate-paused", "auth-authenticate-refresh", "auth-authenticate-untrusted-refresh",
		"auth-authenticate-stale-token", "auth-token-cookie-rotation", "auth-authenticate-validation-201",
		"auth-authenticate-refresh-202", "auth-authenticate-empty-headers", "auth-authenticate-empty-headers-refresh",
		"auth-authenticate-quoted-cookie", "auth-authenticate-quoted-cookie-rotation", "auth-authenticate-explicit-cookie"} {
		t.Run(name, func(t *testing.T) { t.Parallel(); nativeSRPReplay(t, name) })
	}
}
