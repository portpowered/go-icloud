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
	case authChallengeCommand:
		return authResult(client.GetAuthenticationChallenge(ctx, icloud.NativeAuthRequest{Auth: state.Auth, State: state}))
	case authExistingCodeCommand:
		return authResult(client.UseExistingTrustedDeviceCode(ctx, icloud.NativeAuthRequest{Auth: state.Auth, State: state}))
	case authPCSCommand:
		return authResult(client.RequestPCSAccess(ctx, icloud.RequestPCSAccessRequest{
			Auth: state.Auth, State: state, Service: config.authService}))
	case authTrustCommand:
		return authResult(client.TrustSession(ctx, icloud.NativeAuthRequest{Auth: state.Auth, State: state}))
	case authSecurityKeyCommand:
		return authResult(client.ConfirmSecurityKey(ctx, icloud.ConfirmSecurityKeyRequest{
			Auth: state.Auth, State: state, DeviceID: config.securityKeyID}))
	case authSecurityKeyAssertionCommand:
		return verifySecurityKeyAssertion(ctx, client, config.requestFile, state)
	case authMFARequestCommand:
		var phone *icloud.TrustedPhoneNumberID

		if config.phoneID != "" {
			selected, err := selectedPhone(state, config.phoneID)
			if err != nil {
				return nil, err
			}

			phone = selected
		}

		return authResult(client.RequestTwoFactorCode(ctx, icloud.RequestTwoFactorCodeRequest{
			Auth: state.Auth, State: state, PhoneNumberID: phone}))
	case authMFAVerifyCommand:
		code, err := readSecret(ctx, input, environment, "GO_ICLOUD_CODE", config.secretStdin)
		if err != nil {
			return nil, err
		}

		return authResult(client.VerifyTwoFactorCode(ctx, icloud.VerifyTwoFactorCodeRequest{
			Auth: state.Auth, State: state, Code: code}))
	case authMFASendTwoStepCommand, authMFAVerifyTwoStepCommand:
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
	for _, phone := range state.Challenge.PhoneNumbers {
		numeric, numericErr := phone.ID.AsTrustedPhoneNumberID0()
		text, textErr := phone.ID.AsTrustedPhoneNumberID1()

		if (numericErr == nil && strconv.Itoa(numeric) == identifier) || (textErr == nil && text == identifier) {
			return &phone.ID, nil
		}
	}

	return nil, errTrustedPhone
}
