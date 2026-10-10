package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/commandmodels"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

var (
	errAccountEnvironment = errors.New("set GO_ICLOUD_ACCOUNT for first login")
	errAuthRefused        = errors.New("the authentication step was not accepted")
	errTrustedDevice      = errors.New("choose --trusted-device from mfa-devices")
	errTrustedPhone       = errors.New("choose --phone-id from auth-challenge")
	errAccountMismatch    = errors.New("saved credentials belong to another account; select another --session file")
)

type authSummary = commandmodels.AuthenticationSummary

func authenticateCommand(ctx context.Context, client icloud.Client, config options,
	input io.ReadCloser, environment Environment, output io.Writer,
) error {
	if config.operation == authLoginCommand || config.operation == authRenewCommand {
		return login(ctx, client, config, input, environment, output)
	}
	if config.operation == authSecurityKeysCommand {
		result, err := client.ListSecurityKeyDevices(ctx, icloud.ListSecurityKeyDevicesRequest{})
		if err != nil {
			return fmt.Errorf("list local security keys: %w", err)
		}

		return writeResult(output, result)
	}

	state, err := loadNativeAuthentication(config.session)
	if err != nil {
		return err
	}

	switch config.operation {
	case authBridgeCommand:
		return verifyBridge(ctx, client, config, state, input, environment, output)
	case authStatusCommand:
		return authenticationStatus(ctx, client, config, state, output)
	case authLogoutCommand:
		return logout(ctx, client, config, state, output)
	case authMFADevicesCommand:
		return listTrustedDevices(ctx, client, config, state, output)
	default:
		result, operationErr := nativeAuthStep(ctx, client, config, state, input, environment)
		if operationErr != nil {
			return fmt.Errorf("authentication: %w", operationErr)
		}

		return storeAuthenticationResult(ctx, config.session, result, output)
	}
}

func loadNativeAuthentication(path string) (icloud.NativeAuthState, error) {
	var state icloud.NativeAuthState

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return state, &SessionError{Cause: err}
	}

	err = json.Unmarshal(data, &state)
	if err != nil {
		return state, &SessionError{Cause: err}
	}

	if state.Auth.ClientID == "" {
		state.Auth, err = loadSession(path)
		if err != nil {
			return state, err
		}
	}

	return state, nil
}

func storeAuthenticationResult(ctx context.Context, path string, result *icloud.NativeAuthResult,
	output io.Writer,
) error {
	err := saveNativeAuthentication(ctx, path, result.State)
	if err != nil {
		return err
	}

	phones, err := visiblePhones(result.State.Challenge.PhoneNumbers)
	if err != nil {
		return err
	}

	summary := authSummary{SessionFile: path, Success: result.Success, Trusted: result.TrustedSession,
		RequiresTwoFactor: result.RequiresTwoFactor, RequiresTwoStep: result.RequiresTwoStep,
		CodeRequested: result.State.CodeRequested, DeliveryMethod: string(result.State.DeliveryMethod),
		DeliveryNotice: result.State.DeliveryNotice,
		PhoneNumbers:   phones, SecurityKeyNames: append([]string{}, result.State.Challenge.SecurityKeyNames...)}

	err = writeResult(output, summary)
	if err != nil {
		return err
	}

	if !result.Success {
		return errAuthRefused
	}

	return nil
}

func saveNativeAuthentication(ctx context.Context, path string, state icloud.NativeAuthState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return &SessionError{Cause: err}
	}

	err = savePrivateData(ctx, path, data)
	if err != nil {
		return &SessionError{Cause: err}
	}

	return nil
}
