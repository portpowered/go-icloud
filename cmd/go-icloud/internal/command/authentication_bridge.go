package command

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

var errBridgeMissing = errors.New("the trusted-device bridge did not return a session")

func verifyBridge(ctx context.Context, client icloud.Client, config options,
	state icloud.NativeAuthState, input io.ReadCloser, environment Environment, output io.Writer,
) (resultErr error) {
	session, openErr := client.OpenNativeBridgeSession(ctx, icloud.OpenNativeBridgeSessionRequest{Auth: state.Auth, State: state})
	if session == nil {
		return fmt.Errorf("open trusted-device bridge: %w", errors.Join(errBridgeMissing, openErr))
	}
	defer func() { resultErr = errors.Join(resultErr, session.Close()) }()
	progress, err := session.State()
	if err != nil {
		return errors.Join(openErr, fmt.Errorf("trusted-device bridge state: %w", err))
	}
	if err = saveNativeAuthentication(ctx, config.session, progress.State); err != nil {
		return err
	}
	if openErr != nil {
		return fmt.Errorf("open trusted-device bridge: %w", openErr)
	}
	code, err := readSecret(ctx, input, environment, "GO_ICLOUD_CODE", config.secretStdin)
	if err != nil {
		return err
	}
	result, err := session.VerifyCode(ctx, icloud.VerifyNativeBridgeCodeRequest{Code: code})
	if err != nil {
		return fmt.Errorf("verify trusted-device bridge: %w", err)
	}
	return storeAuthenticationResult(ctx, config.session, result, output)
}
