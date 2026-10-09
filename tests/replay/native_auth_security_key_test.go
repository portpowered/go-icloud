package replay_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencies/securitykey"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
	keymodels "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

func TestNativeSecurityKeyAssertionReplay(t *testing.T) {
	t.Parallel()
	for _, ceremony := range []bool{false, true} {
		t.Run(map[bool]string{false: "caller assertion", true: "device ceremony"}[ceremony], func(t *testing.T) {
			t.Parallel()
			raw, transport, state := nativeFlowFixture(t, "auth-security-key-assertion-accepted")
			assertion := nativeFixtureAssertion(t, raw, &state)
			provider := &nativeFixtureAuthenticator{devices: []icloud.SecurityKeyDevice{{ID: "synthetic-device", Name: "Synthetic key"}}, assertion: assertion, err: nil, request: nil, mutateInput: true}
			client, err := icloud.New(icloud.WithHTTPTransport(transport), icloud.WithSecurityKeyAuthenticator(provider))
			if err != nil {
				t.Fatal(err)
			}
			before, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			var result *icloud.NativeAuthResult
			if ceremony {
				result, err = client.ConfirmSecurityKey(t.Context(), icloud.ConfirmSecurityKeyRequest{Auth: state.Auth, State: state, DeviceID: ""})
			} else {
				result, err = client.VerifySecurityKey(t.Context(), icloud.VerifySecurityKeyRequest{Auth: state.Auth, State: state, Assertion: assertion})
			}
			if err != nil {
				t.Fatal(err)
			}
			nativeAssertState(t, raw, result)
			after, err := json.Marshal(state)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("security-key verification changed caller state")
			}
			if ceremony && (provider.request == nil || provider.request.DeviceID != "synthetic-device" || provider.request.Origin != icloud.HttpsappleCom || provider.request.UserVerification != icloud.Discouraged) {
				t.Fatal("security-key ceremony binding differs")
			}
			if err = transport.AssertConsumed(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func nativeFixtureAssertion(t *testing.T, raw map[string]json.RawMessage, state *icloud.NativeAuthState) icloud.SecurityKeyAssertion {
	t.Helper()
	initial := authReplayObjectBytes(t, raw["initial_state"])
	var challenge auth.AuthChallenge
	authReplayDecode(t, initial["auth_data"], &challenge)
	if challenge.FsaChallenge == nil {
		t.Fatal("security challenge missing")
	}
	key := challenge.FsaChallenge
	state.Challenge.SecurityKeyChallenge = &icloud.SecurityKeyChallenge{Challenge: *key.Challenge, CredentialIDs: append([]string{}, *key.KeyHandles...), RelyingPartyID: *key.RpId}
	state.Challenge.SecurityKeyNames = append([]string{}, *challenge.KeyNames...)
	state.Challenge.ProviderData = bytes.Clone(initial["auth_data"])
	var flow []struct {
		Inputs []json.RawMessage `json:"inputs"`
	}
	authReplayDecode(t, raw["inputs"], &flow)
	var payload auth.AuthWebAuthnAssertion
	authReplayDecode(t, flow[0].Inputs[0], &payload)
	return icloud.SecurityKeyAssertion{ClientData: payload.ClientData, Signature: payload.SignatureData,
		AuthenticatorData: payload.AuthenticatorData, CredentialID: payload.CredentialID, UserHandle: payload.UserHandle}
}

type nativeFixtureAuthenticator struct {
	devices     []icloud.SecurityKeyDevice
	assertion   icloud.SecurityKeyAssertion
	err         error
	request     *icloud.SecurityKeyCeremony
	mutateInput bool
}

func (provider *nativeFixtureAuthenticator) Devices(_ context.Context) ([]icloud.SecurityKeyDevice, error) {
	return provider.devices, nil
}
func (provider *nativeFixtureAuthenticator) Assert(_ context.Context, request icloud.SecurityKeyCeremony) (icloud.SecurityKeyAssertion, error) {
	provider.request = &request
	if provider.mutateInput {
		request.Challenge.CredentialIDs[0] = "mutation-probe"
	}
	return provider.assertion, provider.err
}

func TestNativeSecurityKeyBindingRejectsInvalidAssertions(t *testing.T) {
	t.Parallel()
	for _, control := range []string{"challenge", "origin", "client type", "cross origin", "credential", "rp hash", "empty signature", "short authenticator"} {
		t.Run(control, func(t *testing.T) {
			t.Parallel()
			raw, _, state := nativeFlowFixture(t, "auth-security-key-assertion-accepted")
			assertion := nativeFixtureAssertion(t, raw, &state)
			switch control {
			case "challenge":
				state.Challenge.SecurityKeyChallenge.Challenge = "d3Jvbmc"
			case "origin":
				assertion.ClientData = bytes.ReplaceAll(assertion.ClientData, []byte("https://apple.com"), []byte("https://wrong.example.invalid"))
			case "client type":
				assertion.ClientData = bytes.ReplaceAll(assertion.ClientData, []byte("webauthn.get"), []byte("webauthn.create"))
			case "cross origin":
				assertion.ClientData = bytes.ReplaceAll(assertion.ClientData, []byte("false"), []byte("true"))
			case "credential":
				assertion.CredentialID = []byte("wrong")
			case "rp hash":
				assertion.AuthenticatorData[0] ^= 1
			case "empty signature":
				assertion.Signature = nil
			case "short authenticator":
				assertion.AuthenticatorData = assertion.AuthenticatorData[:32]
			}
			client, err := icloud.New(icloud.WithHTTPTransport(nativeNoAuthTransport{test: t}))
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.VerifySecurityKey(t.Context(), icloud.VerifySecurityKeyRequest{Auth: state.Auth, State: state, Assertion: assertion})
			var failure *icloud.ClientError
			if !errors.As(err, &failure) || failure.Kind() != icloud.Configuration {
				t.Fatal("invalid assertion was not rejected before HTTP")
			}
		})
	}
}

type nativeNoAuthTransport struct{ test *testing.T }

func (transport nativeNoAuthTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	transport.test.Fatal("unexpected authentication HTTP traffic")
	return nil, errors.ErrUnsupported
}

func TestNativeSecurityKeyProviderErrorsRemainInspectable(t *testing.T) {
	t.Parallel()
	for _, control := range []struct {
		name  string
		cause error
		kind  icloud.ErrorKind
	}{
		{name: "cancellation", cause: context.Canceled, kind: icloud.Canceled},
		{name: "deadline", cause: context.DeadlineExceeded, kind: icloud.Timeout},
		{name: "PIN required", cause: securitykey.ErrPINRequired, kind: icloud.AuthenticationRequired},
		{name: "unsupported", cause: securitykey.ErrUnsupported, kind: icloud.Configuration},
		{name: "invalid device proof", cause: securitykey.ErrProtocol, kind: icloud.InvalidResponse},
		{name: "device rejected credential", cause: &securitykey.Error{Stage: "CTAP", Status: int(keymodels.NoCredentials), Cause: securitykey.ErrProtocol}, kind: icloud.Provider},
		{name: "device access", cause: errors.ErrUnsupported, kind: icloud.Transport},
	} {
		t.Run(control.name, func(t *testing.T) {
			t.Parallel()
			raw, _, state := nativeFlowFixture(t, "auth-security-key-assertion-accepted")
			assertion := nativeFixtureAssertion(t, raw, &state)
			provider := &nativeFixtureAuthenticator{devices: []icloud.SecurityKeyDevice{{ID: "synthetic-device", Name: "Synthetic key"}}, assertion: assertion, err: control.cause, request: nil, mutateInput: false}
			client, err := icloud.New(icloud.WithHTTPTransport(nativeNoAuthTransport{test: t}), icloud.WithSecurityKeyAuthenticator(provider))
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.ConfirmSecurityKey(t.Context(), icloud.ConfirmSecurityKeyRequest{Auth: state.Auth, State: state, DeviceID: "synthetic-device"})
			var failure *icloud.ClientError
			if !errors.As(err, &failure) || failure.Kind() != control.kind || !errors.Is(err, control.cause) {
				t.Fatal("security-key error lost class or cause")
			}
		})
	}
}
