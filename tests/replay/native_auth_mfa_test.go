package replay_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

func TestNativeMFAReplay(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"auth-2fa-trusted-device-accepted", "auth-2fa-trusted-device-accepted-conflict",
		"auth-2fa-trusted-device-refused", "auth-2fa-trusted-device-rejected-conflict",
		"auth-sms-validate-accepted", "auth-sms-validate-refused", "auth-sms-request", "auth-sms-request-refused",
		"auth-trust-success", "auth-trust-refused", "auth-trust-untrusted-account",
		"auth-send-code-true", "auth-send-code-false", "auth-send-code-error",
		"auth-validate-code-success", "auth-validate-code-wrong", "auth-validate-code-provider-error"} {
		t.Run(name, func(t *testing.T) { t.Parallel(); nativeMFAReplay(t, name) })
	}
}

func nativeMFAReplay(t *testing.T, name string) {
	t.Helper()
	raw, transport, state := nativeFlowFixture(t, name)
	nativeFixtureMFAState(t, raw, &state)
	before, marshalErr := json.Marshal(state)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}
	result, err := nativeMFAOperation(t, client, raw, state)

	nativeFlowExpectedError(t, raw, err)
	if err == nil {
		nativeAssertState(t, raw, result)
		expected := authReplayObjectBytes(t, raw["result"])

		var accepted bool

		authReplayDecode(t, expected["value"], &accepted)
		if result.Success != accepted {
			t.Fatal("authentication step acceptance differs")
		}
	}
	after, marshalErr := json.Marshal(state)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("authentication changed caller-owned state")
	}
	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func nativeMFAOperation(t *testing.T, client *icloud.SDK,
	raw map[string]json.RawMessage, state icloud.NativeAuthState,
) (*icloud.NativeAuthResult, error) {
	t.Helper()

	var operation string

	var inputs []json.RawMessage

	authReplayDecode(t, raw["operation"], &operation)
	authReplayDecode(t, raw["inputs"], &inputs)

	ctx := t.Context()

	var (
		result *icloud.NativeAuthResult
		err    error
	)

	switch operation {
	case nativeValidateTwoFactorOperation:
		var code string

		authReplayDecode(t, inputs[0], &code)

		result, err = client.VerifyTwoFactorCode(ctx, icloud.VerifyTwoFactorCodeRequest{
			Auth: state.Auth, State: state, Code: code})
	case nativeRequestTwoFactorOperation:
		result, err = client.RequestTwoFactorCode(ctx, icloud.RequestTwoFactorCodeRequest{
			Auth: state.Auth, State: state, PhoneNumberID: nil})
	case "trust_session":
		result, err = client.TrustSession(ctx, icloud.NativeAuthRequest{Auth: state.Auth, State: state})
	case "send_verification_code":
		result, err = client.SendTwoStepCode(ctx, icloud.SendTwoStepCodeRequest{
			Auth: state.Auth, State: state, Device: nativeFixtureDevice(t, inputs[0])})
	case "validate_verification_code":
		var code string

		authReplayDecode(t, inputs[1], &code)

		result, err = client.VerifyTwoStepCode(ctx, icloud.VerifyTwoStepCodeRequest{Auth: state.Auth, State: state,
			Device: nativeFixtureDevice(t, inputs[0]), Code: code})
	default:
		t.Fatalf("unknown native MFA fixture operation %s", operation)

		return nil, errors.ErrUnsupported
	}

	if err != nil {
		return nil, fmt.Errorf("native MFA fixture operation: %w", err)
	}

	return result, nil
}

func nativeFixtureDevice(t *testing.T, raw json.RawMessage) icloud.TrustedAuthDevice {
	t.Helper()

	var device auth.AuthTrustedDevice

	authReplayDecode(t, raw, &device)
	if device.Id == nil {
		t.Fatal("fixture device identity is missing")
	}
	return icloud.TrustedAuthDevice{ID: *device.Id, Metadata: bytes.Clone(raw)}
}

func nativeFixtureMFAState(t *testing.T, raw map[string]json.RawMessage, state *icloud.NativeAuthState) {
	t.Helper()
	initial := authReplayObjectBytes(t, raw["initial_state"])
	if delivery, exists := initial["delivery_method"]; exists {
		authReplayDecode(t, delivery, &state.DeliveryMethod)
	}
	if requested, exists := initial["code_requested"]; exists {
		authReplayDecode(t, requested, &state.CodeRequested)
	}
	if required, exists := initial["requires_mfa"]; exists {
		authReplayDecode(t, required, &state.RequiresMFA)
	}
	if challenge, exists := initial["auth_data"]; exists {
		state.Challenge.ProviderData = bytes.Clone(challenge)

		var data auth.AuthChallenge

		authReplayDecode(t, challenge, &data)
		if data.Mode != nil {
			state.Challenge.Mode = *data.Mode
		}
		phones := []auth.AuthTrustedPhoneNumber{}
		if data.TrustedPhoneNumber != nil {
			phones = append(phones, *data.TrustedPhoneNumber)
		}
		if data.PhoneNumberVerification != nil && data.PhoneNumberVerification.TrustedPhoneNumber != nil {
			phones = append(phones, *data.PhoneNumberVerification.TrustedPhoneNumber)
		}

		for _, phone := range phones {
			state.Challenge.PhoneNumbers = append(state.Challenge.PhoneNumbers, nativeFixturePhone(t, phone))
		}
	}
}

func nativeFixturePhone(t *testing.T, phone auth.AuthTrustedPhoneNumber) icloud.TrustedPhoneNumber {
	t.Helper()
	encoded, err := json.Marshal(phone.Id)
	if err != nil {
		t.Fatal(err)
	}

	var identifier icloud.TrustedPhoneNumberID

	authReplayDecode(t, encoded, &identifier)
	result := icloud.TrustedPhoneNumber{ID: identifier, Number: "", PushMode: "", NonFTEU: phone.NonFTEU}
	if phone.PushMode != nil {
		result.PushMode = *phone.PushMode
	}
	if phone.NumberWithDialCode != nil {
		result.Number = *phone.NumberWithDialCode
	}
	return result
}

func TestNativeAuthenticationStatusReplay(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"auth-status-trusted", "auth-status-untrusted", "auth-status-rejected",
		"auth-status-no-token", "auth-status-no-cookie"} {
		t.Run(name, func(t *testing.T) { t.Parallel(); nativeStatusReplay(t, name) })
	}
}

func nativeStatusReplay(t *testing.T, name string) {
	t.Helper()
	raw, transport, state := nativeFlowFixture(t, name)
	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.GetAuthenticationStatus(t.Context(), icloud.NativeAuthRequest{Auth: state.Auth, State: state})
	if err != nil {
		t.Fatal(err)
	}
	expected := authReplayObjectBytes(t, raw["result"])
	value := authReplayObjectBytes(t, expected["value"])
	actual := map[string]bool{"authenticated": result.Authenticated, "trusted_session": result.TrustedSession,
		"requires_2fa": result.RequiresTwoFactor, "requires_2sa": result.RequiresTwoStep}

	var expectedFlags map[string]bool

	authReplayDecode(t, expected["value"], &expectedFlags)
	if !reflect.DeepEqual(actual, expectedFlags) {
		t.Fatalf("auth status differs: actual %v expected %v", actual, value)
	}

	nativeAssertState(t, raw, &icloud.NativeAuthResult{State: result.State, TrustedSession: result.TrustedSession,
		RequiresTwoFactor: result.RequiresTwoFactor, RequiresTwoStep: result.RequiresTwoStep,
		Responses: result.Responses, Success: result.Authenticated})
	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}
