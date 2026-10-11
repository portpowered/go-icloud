package command

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

var errSecurityKeyLogin = errors.New("use mfa-security-keys and mfa-security-key with the saved session")
var errNoSMSVerification = errors.New("Apple supplied no SMS route; use auth challenge or auth bridge with this session")

func completeInteractiveLogin(ctx context.Context, client icloud.Client, config options,
	result *icloud.NativeAuthResult, input io.ReadCloser, environment Environment, output, diagnostic io.Writer,
) error {
	err := saveNativeAuthentication(ctx, config.session, result.State)
	if err != nil {
		return err
	}

	if !result.State.RequiresMFA {
		return storeAuthenticationResult(ctx, config.session, result, output)
	}

	config.secretStdin = environment("GO_ICLOUD_CODE") == ""

	bridge := result.State.DeliveryMethod == icloud.TwoFactorDeliveryTrustedDevice

	if bridge && config.phoneID == "" && len(result.State.Challenge.BridgeBootstrap) > 0 {
		_, _ = fmt.Fprint(diagnostic, "Apple requires trusted-device approval. Opening the prompt; enter its code: ")

		return verifyBridge(ctx, client, config, result.State, input, environment, output)
	}

	if result.State.DeliveryMethod == icloud.TwoFactorDeliverySecurityKey {
		return errSecurityKeyLogin
	}

	if !result.State.CodeRequested {
		phone, err := interactiveSMSPhone(result.State, config.phoneID)
		if err != nil {
			return err
		}

		requested, err := client.RequestTwoFactorCode(ctx, icloud.RequestTwoFactorCodeRequest{
			Auth: result.State.Auth, State: result.State, PhoneNumberID: phone,
		})
		if err != nil {
			return fmt.Errorf("request verification code: %w", err)
		}

		err = saveNativeAuthentication(ctx, config.session, requested.State)
		if err != nil {
			return err
		}

		if !requested.Success || !requested.State.CodeRequested {
			return errNoSMSVerification
		}

		result = requested
	}

	_, _ = fmt.Fprintf(diagnostic, "Apple verification code requested (%s). Enter the code: ", result.State.DeliveryMethod)

	code, err := readSecret(ctx, input, environment, "GO_ICLOUD_CODE", config.secretStdin)
	if err != nil {
		return err
	}

	verified, err := client.VerifyTwoFactorCode(ctx, icloud.VerifyTwoFactorCodeRequest{
		Auth: result.State.Auth, State: result.State, Code: code,
	})
	if err != nil {
		return fmt.Errorf("verify code: %w", err)
	}

	return storeAuthenticationResult(ctx, config.session, verified, output)
}

func interactiveSMSPhone(state icloud.NativeAuthState, identifier string) (*icloud.TrustedPhoneNumberID, error) {
	if identifier != "" {
		return selectedPhone(state, identifier)
	}

	if state.Challenge.Mode == "" || state.Challenge.Mode == "sms" {
		for _, phone := range state.Challenge.PhoneNumbers {
			if state.Challenge.Mode == "sms" || phone.PushMode == "sms" {
				return &phone.ID, nil
			}
		}
	}

	return nil, errNoSMSVerification
}
