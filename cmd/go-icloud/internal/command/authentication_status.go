package command

import (
	"context"
	"fmt"
	"io"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/commandmodels"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

func authenticationStatus(ctx context.Context, client icloud.Client, config options,
	state icloud.NativeAuthState, output io.Writer,
) error {
	result, err := client.GetAuthenticationStatus(ctx, icloud.NativeAuthRequest{Auth: state.Auth, State: state})
	if err != nil {
		return fmt.Errorf("authentication status: %w", err)
	}
	if err = saveNativeAuthentication(ctx, config.session, result.State); err != nil {
		return err
	}
	return writeResult(output, commandmodels.AuthenticationStatusSummary{SessionFile: config.session,
		Authenticated: result.Authenticated, Trusted: result.TrustedSession,
		RequiresTwoFactor: result.RequiresTwoFactor, RequiresTwoStep: result.RequiresTwoStep})
}
