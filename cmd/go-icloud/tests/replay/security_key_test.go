package replay_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

type commandSecurityKey struct {
	assertion icloud.SecurityKeyAssertion
	request   *icloud.SecurityKeyCeremony
	failure   error
}

func (*commandSecurityKey) Devices(context.Context) ([]icloud.SecurityKeyDevice, error) {
	return []icloud.SecurityKeyDevice{{ID: "synthetic-device", Name: "Synthetic key"}}, nil
}

func (provider *commandSecurityKey) Assert(_ context.Context,
	request icloud.SecurityKeyCeremony,
) (icloud.SecurityKeyAssertion, error) {
	provider.request = &request
	return provider.assertion, provider.failure
}

func TestSecurityKeyCommandUsesSelectedDeviceAndSourceAssertion(t *testing.T) {
	t.Parallel()
	for _, control := range []string{"accepted", "caller assertion", "missing device", "cancelled ceremony"} {
		t.Run(control, func(t *testing.T) {
			t.Parallel()
			raw := readObject(t, "../../../../tests/replay/fixtures/synthetic/http/auth-security-key-assertion-accepted.json")
			var exchanges []replay.Exchange
			decode(t, raw["exchanges"], &exchanges)
			transport, err := replay.NewHTTPTransport(exchanges)
			if err != nil {
				t.Fatal(err)
			}
			state := nativeCommandState(t, raw["initial_state"])
			provider := &commandSecurityKey{assertion: commandKeyAssertion(t, raw, &state), request: nil, failure: nil}
			if control == "cancelled ceremony" {
				provider.failure = context.Canceled
			}
			client, err := icloud.New(icloud.WithHTTPTransport(transport), icloud.WithSecurityKeyAuthenticator(provider))
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "session.json")
			if err = os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			identifier := "synthetic-device"
			if control == "missing device" {
				identifier = "unavailable"
			}
			var output, diagnostic bytes.Buffer
			arguments := commandKeyArguments(t, control, path, identifier, provider.assertion)
			err = command.RunWithInput(t.Context(), client, arguments,
				io.NopCloser(strings.NewReader("")), nil, &output, &diagnostic)
			commandKeyOutcome(t, control, err, provider, transport)
			saved, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if control != "accepted" && control != "caller assertion" && !bytes.Equal(saved, data) {
				t.Fatal("failed hardware ceremony changed saved credentials")
			}
			for _, private := range []string{"synthetic-token", "synthetic-trust", "synthetic-cookie", "credentialIDs", "accountData", "responses"} {
				if strings.Contains(output.String()+diagnostic.String(), private) {
					t.Fatal("security-key console disclosed private state")
				}
			}
		})
	}
}

func commandKeyArguments(t *testing.T, control, path, identifier string,
	assertion icloud.SecurityKeyAssertion,
) []string {
	t.Helper()
	if control != "caller assertion" {
		return []string{sessionFlag, path, "--security-key", identifier, "mfa-security-key"}
	}
	data, err := json.Marshal(icloud.VerifySecurityKeyRequest{Auth: icloud.AuthContext{ClientID: "foreign"},
		State: icloud.NativeAuthState{}, Assertion: assertion})
	if err != nil {
		t.Fatal(err)
	}
	request := filepath.Join(t.TempDir(), "assertion.json")
	if err = os.WriteFile(request, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return []string{sessionFlag, path, "--request", request, "mfa-security-key-assertion"}
}

func commandKeyOutcome(t *testing.T, control string, err error,
	provider *commandSecurityKey, transport *replay.HTTPTransport,
) {
	t.Helper()
	if control != "accepted" && control != "caller assertion" {
		if err == nil {
			t.Fatal("invalid hardware ceremony succeeded")
		}
		if control == "missing device" && provider.request != nil {
			t.Fatal("unavailable device reached assertion ceremony")
		}
		if control == "cancelled ceremony" && !errors.Is(err, context.Canceled) {
			t.Fatal("hardware cancellation cause was lost")
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if control == "accepted" && (provider.request == nil || provider.request.DeviceID != "synthetic-device" ||
		provider.request.Origin != icloud.HttpsappleCom || provider.request.UserVerification != icloud.Discouraged) {
		t.Fatal("CLI selected ceremony differs from generated security-key contract")
	}
	if err = transport.AssertConsumed(); err != nil {
		t.Fatal(err)
	}
}

func commandKeyAssertion(t *testing.T, raw map[string]json.RawMessage,
	state *icloud.NativeAuthState,
) icloud.SecurityKeyAssertion {
	t.Helper()
	initial := readRawObject(t, raw["initial_state"])
	var challenge auth.AuthChallenge
	decode(t, initial["auth_data"], &challenge)
	if challenge.FsaChallenge == nil {
		t.Fatal("Source security-key challenge is absent")
	}
	key := challenge.FsaChallenge
	state.Challenge.SecurityKeyChallenge = &icloud.SecurityKeyChallenge{
		Challenge: *key.Challenge, CredentialIDs: append([]string{}, *key.KeyHandles...), RelyingPartyID: *key.RpId}
	state.Challenge.SecurityKeyNames = append([]string{}, *challenge.KeyNames...)
	state.Challenge.ProviderData = bytes.Clone(initial["auth_data"])
	var flow []struct {
		Inputs []json.RawMessage `json:"inputs"`
	}
	decode(t, raw["inputs"], &flow)
	var assertion auth.AuthWebAuthnAssertion
	decode(t, flow[0].Inputs[0], &assertion)
	return icloud.SecurityKeyAssertion{ClientData: assertion.ClientData, Signature: assertion.SignatureData,
		AuthenticatorData: assertion.AuthenticatorData, CredentialID: assertion.CredentialID, UserHandle: assertion.UserHandle}
}
