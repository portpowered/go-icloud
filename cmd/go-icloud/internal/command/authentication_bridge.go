package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

var errBridgeMissing = errors.New("the trusted-device bridge did not return a session")

func verifyBridge(ctx context.Context, client icloud.Client, config options,
	state icloud.NativeAuthState, input io.ReadCloser, environment Environment, output io.Writer,
) (resultErr error) {
	request := icloud.OpenNativeBridgeSessionRequest{Auth: state.Auth, State: state}
	session, openErr := client.OpenNativeBridgeSession(ctx, request)
	if session == nil {
		return fmt.Errorf("open trusted-device bridge: %w", errors.Join(errBridgeMissing, openErr))
	}
	defer func() {
		resultErr = errors.Join(resultErr, persistBridgeAuthentication(ctx, config.session, session), session.Close())
	}()
	if err := persistBridgeAuthentication(ctx, config.session, session); err != nil {
		return errors.Join(openErr, err)
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

// Persist rotated credentials even when the command deadline or hardware input has failed.
// Local persistence has its own short bound; socket operations retain the command context.
func persistBridgeAuthentication(ctx context.Context, path string, session *icloud.NativeBridgeSession) error {
	progress, err := session.State()
	if err != nil {
		return fmt.Errorf("trusted-device bridge persistence state: %w", err)
	}
	persistContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return saveNativeAuthentication(persistContext, path, progress.State)
}
