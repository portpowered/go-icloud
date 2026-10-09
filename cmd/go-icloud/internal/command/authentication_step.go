package command

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func nativeAuthStep(ctx context.Context, client icloud.Client, config options, state icloud.NativeAuthState,
	input io.ReadCloser, environment Environment,
) (*icloud.NativeAuthResult, error) {
	switch config.operation {
	case "auth-challenge":
		return authResult(client.GetAuthenticationChallenge(ctx, icloud.NativeAuthRequest{Auth: state.Auth, State: state}))
	case "mfa-existing-code":
		return authResult(client.UseExistingTrustedDeviceCode(ctx, icloud.NativeAuthRequest{Auth: state.Auth, State: state}))
	case "pcs-access":
		return authResult(client.RequestPCSAccess(ctx, icloud.RequestPCSAccessRequest{
			Auth: state.Auth, State: state, Service: config.authService}))
	case "trust":
		return authResult(client.TrustSession(ctx, icloud.NativeAuthRequest{Auth: state.Auth, State: state}))
	case "mfa-security-key":
		return authResult(client.ConfirmSecurityKey(ctx, icloud.ConfirmSecurityKeyRequest{
			Auth: state.Auth, State: state, DeviceID: config.securityKeyID}))
	case "mfa-security-key-assertion":
		return verifySecurityKeyAssertion(ctx, client, config.requestFile, state)
	case "mfa-request":
		phone, err := selectedPhone(state, config.phoneID)
		if err != nil {
			return nil, err
		}

		return authResult(client.RequestTwoFactorCode(ctx, icloud.RequestTwoFactorCodeRequest{
			Auth: state.Auth, State: state, PhoneNumberID: phone}))
	case "mfa-verify":
		code, err := readSecret(ctx, input, environment, "GO_ICLOUD_CODE", config.secretStdin)
		if err != nil {
			return nil, err
		}

		return authResult(client.VerifyTwoFactorCode(ctx, icloud.VerifyTwoFactorCodeRequest{
			Auth: state.Auth, State: state, Code: code}))
	case "mfa-send-two-step", "mfa-verify-two-step":
		return twoStepCommand(ctx, client, config, state, input, environment)
	default:
		return nil, errCommand
	}
}

func verifySecurityKeyAssertion(ctx context.Context, client icloud.Client,
	path string, state icloud.NativeAuthState,
) (*icloud.NativeAuthResult, error) {
	request, err := readWriteRequest[icloud.VerifySecurityKeyRequest](ctx, path)
	if err != nil {
		return nil, err
	}
	request.Auth = state.Auth
	request.State = state
	return authResult(client.VerifySecurityKey(ctx, *request))
}

func authResult(result *icloud.NativeAuthResult, err error) (*icloud.NativeAuthResult, error) {
	if err != nil {
		return nil, fmt.Errorf("SDK authentication: %w", err)
	}

	return result, nil
}

func selectedPhone(state icloud.NativeAuthState, identifier string) (*icloud.TrustedPhoneNumberID, error) {
	if identifier == "" {
		return nil, nil
	}

	for _, phone := range state.Challenge.PhoneNumbers {
		numeric, numericErr := phone.ID.AsTrustedPhoneNumberID0()
		text, textErr := phone.ID.AsTrustedPhoneNumberID1()
		if (numericErr == nil && strconv.Itoa(numeric) == identifier) || (textErr == nil && text == identifier) {
			return &phone.ID, nil
		}
	}

	return nil, errTrustedPhone
}
