package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/commandmodels"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

var errLogoutIncomplete = errors.New("local credentials cleared; remote logout was not confirmed")

type logoutSummary = commandmodels.LogoutSummary

func logout(ctx context.Context, client icloud.Client, config options,
	state icloud.NativeAuthState, output io.Writer,
) error {
	result, err := client.Logout(ctx, icloud.LogoutRequest{Auth: state.Auth, State: state,
		KeepTrusted: config.keepTrusted, AllSessions: config.allSessions, PreserveLocalSession: false})
	if err != nil {
		return fmt.Errorf("logout: %w", err)
	}

	if result.LocalCleared {
		err = os.Remove(filepath.Clean(config.session))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return &SessionError{Cause: err}
		}
	} else {
		err = saveNativeAuthentication(ctx, config.session, result.State)
		if err != nil {
			return err
		}
	}

	err = writeResult(output, logoutSummary{RemoteConfirmed: result.RemoteConfirmed, LocalCleared: result.LocalCleared})
	if err != nil {
		return err
	}

	if !result.RemoteConfirmed {
		return errLogoutIncomplete
	}

	return nil
}
