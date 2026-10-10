package replay_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

const (
	nativeDirectPhoneFixture = "auth-srp-direct-phone-nonfteu-true"
	nativeNestedPhoneFixture = "auth-srp-nested-phone-nonfteu-false"
	nativeKeyDeliveryFixture = "auth-srp-security-key-delivery"
	nativeNonSMSFixture      = "auth-srp-non-sms-delivery"
)

func TestNativeAuthenticationDeliveryEdgesReplay(t *testing.T) {
	t.Parallel()

	for _, name := range []string{nativeDirectPhoneFixture, nativeNestedPhoneFixture,
		nativeKeyDeliveryFixture, nativeNonSMSFixture} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			raw, result := nativeDeliveryEdgeReplay(t, name)
			if name == nativeDirectPhoneFixture || name == nativeNestedPhoneFixture {
				nativeAssertBootstrapPhoneProjection(t, raw, result)
			}
		})
	}
}

func TestNativeTrustSuccessfulStatusRefusalReplay(t *testing.T) {
	t.Parallel()

	nativeMFAReplay(t, "auth-trust-locked-success-status")
	nativeMFAReplay(t, "auth-trust-service-errors-without-reason")
}

func TestNativeTrustLockedAccountPropagationReplay(t *testing.T) {
	t.Parallel()

	raw, transport, state := nativeFlowFixture(t, "auth-trust-locked-forbidden")

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	before, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.TrustSession(t.Context(), icloud.NativeAuthRequest{Auth: state.Auth, State: state})
	nativeFlowExpectedError(t, raw, err)

	var failure *icloud.ClientError

	if result != nil || !errors.As(err, &failure) || failure.Kind() != icloud.AccountLocked {
		t.Fatal("non-OK locked-account refusal lost its typed error")
	}

	after, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(before, after) {
		t.Fatal("failed trust changed caller-owned state")
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func nativeDeliveryEdgeReplay(t *testing.T, name string) (map[string]json.RawMessage, *icloud.NativeAuthResult) {
	t.Helper()

	raw, transport, state := nativeFlowFixture(t, name)
	initial := authReplayObjectBytes(t, raw["initial_state"])

	var password string

	authReplayDecode(t, initial["synthetic_password"], &password)

	client, err := icloud.New(icloud.WithHTTPTransport(transport),
		icloud.WithRandomSource(bytes.NewReader(nativeFixtureEntropy(t, raw))))
	if err != nil {
		t.Fatal(err)
	}

	before, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.Authenticate(t.Context(), icloud.AuthenticateRequest{
		Auth: state.Auth, AccountName: state.AccountName, Password: password, TrustToken: state.TrustToken,
		AccountCountryCode: state.AccountCountryCode, SavedState: &state, ForceRefresh: false,
		PauseTwoFactor: false, Service: nil, AcceptTerms: false,
	})
	if err != nil {
		t.Fatal(err)
	}

	nativeAssertState(t, raw, result)

	after, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(before, after) {
		t.Fatal("authentication delivery changed caller-owned state")
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}

	return raw, result
}

func nativeAssertBootstrapPhoneProjection(t *testing.T, raw map[string]json.RawMessage,
	result *icloud.NativeAuthResult,
) {
	t.Helper()

	expected := authReplayObjectBytes(t, raw["result"])
	state := authReplayObjectBytes(t, expected["auth_state"])

	var challenge auth.AuthChallenge

	authReplayDecode(t, state["challenge"], &challenge)

	if challenge.TrustedPhoneNumber == nil || challenge.PhoneNumberVerification == nil ||
		challenge.PhoneNumberVerification.TrustedPhoneNumbers == nil || challenge.AuthFactors == nil {
		t.Fatal("paired Source challenge is missing phone choices")
	}

	phones := make([]icloud.TrustedPhoneNumber, 0, 1+len(*challenge.PhoneNumberVerification.TrustedPhoneNumbers))

	phones = append(phones, nativeFixturePhone(t, *challenge.TrustedPhoneNumber))
	for _, phone := range *challenge.PhoneNumberVerification.TrustedPhoneNumbers {
		phones = append(phones, nativeFixturePhone(t, phone))
	}

	if !reflect.DeepEqual(result.State.Challenge.PhoneNumbers, phones) ||
		!reflect.DeepEqual(result.State.Challenge.AuthFactors, *challenge.AuthFactors) {
		t.Fatal("public bootstrap phone choices differ from paired Source challenge")
	}

	encoded, err := json.Marshal(challenge.Direct)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(accountJSON(t, result.State.Challenge.BridgeBootstrap), accountJSON(t, encoded)) {
		t.Fatal("public bridge bootstrap lost the phone or application identity")
	}
}
