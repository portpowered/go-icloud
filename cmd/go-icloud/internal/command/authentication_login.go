package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func login(ctx context.Context, client icloud.Client, config options,
	input io.ReadCloser, environment Environment, output io.Writer,
	diagnostic io.Writer,
) error {
	if environment == nil {
		return errAccountEnvironment
	}

	state, err := loadNativeAuthentication(config.session)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	account := environment("GO_ICLOUD_ACCOUNT")
	if account == "" {
		account = state.AccountName
	}

	if account == "" {
		if !config.interactive {
			return errAccountEnvironment
		}

		_, _ = fmt.Fprint(diagnostic, "Apple ID: ")

		account, err = readSecret(ctx, input, environment, "GO_ICLOUD_ACCOUNT", true)
		if err != nil {
			return err
		}
	}

	if state.AccountName != "" && state.AccountName != account {
		return errAccountMismatch
	}

	password := environment("GO_ICLOUD_PASSWORD")
	if config.secretStdin || password != "" || state.Auth.SessionToken == nil {
		if config.interactive && password == "" {
			_, _ = fmt.Fprint(diagnostic, "Password (hidden on a terminal): ")
			config.secretStdin = true
		}

		password, err = readSecret(ctx, input, environment, "GO_ICLOUD_PASSWORD", config.secretStdin)
		if err != nil {
			return err
		}
	}

	if config.china || state.Auth.ChinaMainland == nil {
		state.Auth.ChinaMainland = &config.china
	}

	request := icloud.AuthenticateRequest{Auth: state.Auth, AccountName: account, Password: password,
		SavedState: nil, TrustToken: state.TrustToken, AccountCountryCode: state.AccountCountryCode,
		ForceRefresh: config.forceRefresh || config.operation == authRenewCommand, PauseTwoFactor: !config.interactive,
		AcceptTerms: config.acceptTerms, Service: nil}

	if config.authService != "" {
		request.Service = &config.authService
	}

	if state.Auth.ClientID != "" {
		request.SavedState = &state
	}

	result, err := client.Authenticate(ctx, request)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}

	if config.interactive {
		return completeInteractiveLogin(ctx, client, config, result, input, environment, output, diagnostic)
	}

	return storeAuthenticationResult(ctx, config.session, result, output)
}
