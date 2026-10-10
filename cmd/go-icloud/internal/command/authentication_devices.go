package command

import (
	"context"
	"fmt"
	"io"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/commandmodels"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

type trustedDevicesSummary = commandmodels.TrustedDevicesSummary

func listTrustedDevices(ctx context.Context, client icloud.Client, config options,
	state icloud.NativeAuthState, output io.Writer,
) error {
	result, err := client.ListTrustedDevices(ctx, icloud.NativeAuthRequest{Auth: state.Auth, State: state})
	if err != nil {
		return fmt.Errorf("discover trusted devices: %w", err)
	}

	err = saveNativeAuthentication(ctx, config.session, result.State)
	if err != nil {
		return err
	}

	identifiers := make([]string, 0, len(result.Devices))
	for _, device := range result.Devices {
		identifiers = append(identifiers, device.ID)
	}

	return writeResult(output, trustedDevicesSummary{SessionFile: config.session, DeviceIDs: identifiers})
}

func twoStepCommand(ctx context.Context, client icloud.Client, config options,
	state icloud.NativeAuthState, input io.ReadCloser, environment Environment,
) (*icloud.NativeAuthResult, error) {
	if config.trustedDeviceID == "" {
		return nil, errTrustedDevice
	}

	result, err := client.ListTrustedDevices(ctx, icloud.NativeAuthRequest{Auth: state.Auth, State: state})
	if err != nil {
		return nil, fmt.Errorf("discover trusted devices: %w", err)
	}

	for _, device := range result.Devices {
		if device.ID == config.trustedDeviceID {
			return selectedDeviceCommand(ctx, client, config, result.State, device, input, environment)
		}
	}

	return nil, errTrustedDevice
}

func selectedDeviceCommand(ctx context.Context, client icloud.Client, config options,
	state icloud.NativeAuthState, device icloud.TrustedAuthDevice, input io.ReadCloser, environment Environment,
) (*icloud.NativeAuthResult, error) {
	if config.operation == authMFASendTwoStepCommand {
		return authResult(client.SendTwoStepCode(ctx, icloud.SendTwoStepCodeRequest{
			Auth: state.Auth, State: state, Device: device}))
	}

	code, err := readSecret(ctx, input, environment, "GO_ICLOUD_CODE", config.secretStdin)
	if err != nil {
		return nil, err
	}

	return authResult(client.VerifyTwoStepCode(ctx, icloud.VerifyTwoStepCodeRequest{
		Auth: state.Auth, State: state, Device: device, Code: code}))
}
