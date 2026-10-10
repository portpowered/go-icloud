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
		return errAccountEnvironment
	}

	if state.AccountName != "" && state.AccountName != account {
		return errAccountMismatch
	}

	password := environment("GO_ICLOUD_PASSWORD")
	if config.secretStdin || password != "" || state.Auth.SessionToken == nil {
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
		ForceRefresh: config.forceRefresh || config.operation == authRenewCommand, PauseTwoFactor: true,
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

	return storeAuthenticationResult(ctx, config.session, result, output)
}
