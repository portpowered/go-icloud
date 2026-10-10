package replay_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func TestNativeTermsReplay(t *testing.T) {
	t.Parallel()

	for _, scenario := range []string{"auth-terms-accepted", "auth-terms-missing-version", "auth-terms-refused",
		nativeOneFactorFixture} {
		t.Run(scenario, func(t *testing.T) { t.Parallel(); nativeTermsReplay(t, scenario) })
	}
}

func nativeTermsReplay(t *testing.T, name string) {
	t.Helper()

	raw, transport, state := nativeFlowFixture(t, name)
	request := nativeTermsRequest(t, raw, state, name)

	before, err := json.Marshal(request.SavedState)
	if err != nil {
		t.Fatal(err)
	}
	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Authenticate(t.Context(), request)
	nativeFlowExpectedError(t, raw, err)
	if err == nil {
		expected := authReplayObjectBytes(t, raw["result"])
		final := authReplayObjectBytes(t, expected["auth_state"])
		if !reflect.DeepEqual(accountJSON(t, result.State.AccountData), accountJSON(t, final["account"])) {
			t.Fatal("native terms/login account projection differs from Source")
		}

		nativeFlowResponses(t, raw, result.Responses)
	}
	after, marshalErr := json.Marshal(request.SavedState)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if string(before) != string(after) {
		t.Fatal("native authentication mutated caller state")
	}
	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func nativeTermsRequest(t *testing.T, raw map[string]json.RawMessage, state icloud.NativeAuthState,
	name string,
) icloud.AuthenticateRequest {
	t.Helper()

	initial := authReplayObjectBytes(t, raw["initial_state"])
	request := icloud.AuthenticateRequest{Auth: state.Auth, AccountName: state.AccountName, Password: "",
		AccountCountryCode: state.AccountCountryCode, TrustToken: state.TrustToken, SavedState: &state,
		ForceRefresh: true, AcceptTerms: false, PauseTwoFactor: false, Service: nil}
	if value, exists := initial["accept_terms"]; exists {
		authReplayDecode(t, value, &request.AcceptTerms)
	}
	if value, exists := initial["synthetic_password"]; exists {
		authReplayDecode(t, value, &request.Password)
	}
	if name == nativeOneFactorFixture {
		service := "photos"
		request.Service = &service
	}

	return request
}
