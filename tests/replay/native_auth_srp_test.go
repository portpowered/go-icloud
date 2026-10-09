package replay_test

import (
	"bytes"
	"encoding/base64"
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
	authReplayDecode(t, initial["synthetic_password"], &password)
	var entropy struct {
		RandomBytes []string `json:"random_bytes"`
	}
	if value, exists := raw["entropy"]; exists {
		authReplayDecode(t, value, &entropy)
	}
	var random []byte
	for _, value := range entropy.RandomBytes {
		decoded, decodeErr := base64.StdEncoding.DecodeString(value)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		random = append(random, decoded...)
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
	before, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Authenticate(t.Context(), request)
	nativeFlowExpectedError(t, raw, err)
	if err == nil {
		nativeAssertState(t, raw, result)
	}
	after, marshalErr := json.Marshal(request)
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
	if result.State.RequiresMFA != requires || result.State.CodeRequested != requested || string(result.State.DeliveryMethod) != delivery {
		t.Fatalf("native challenge status differs: %+v", result.State.Challenge)
	}
	projection := icloud.ResumeSessionResult{Auth: result.State.Auth, TrustToken: result.State.TrustToken,
		AccountCountryCode: result.State.AccountCountryCode, AccountData: result.State.AccountData, Responses: result.Responses,
		TrustedSession: result.TrustedSession, RequiresTwoFactor: result.RequiresTwoFactor, RequiresTwoStep: result.RequiresTwoStep}
	assertAuthFlags(t, state, &projection)
	assertResumedCookies(t, state, &projection)
	assertResumedCountry(t, nativeFixtureSession(t, state), &projection)
	nativeFlowResponses(t, raw, result.Responses)
}

func nativeFixtureSession(t *testing.T, state map[string]json.RawMessage) map[string]string {
	t.Helper()
	var session map[string]string
	authReplayDecode(t, state["session_data"], &session)
	return session
}
