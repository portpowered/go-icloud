package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/savedlogin"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

const privateSessionDirectoryMode = 0o700

var errNativeSession = errors.New("native saved-session credentials are unavailable")

type resumeSummary struct {
	SessionFile       string `json:"sessionFile"`
	Trusted           bool   `json:"trusted"`
	RequiresTwoFactor bool   `json:"requiresTwoFactor"`
	RequiresTwoStep   bool   `json:"requiresTwoStep"`
}

func resumeSession(ctx context.Context, client icloud.Client, config options, output io.Writer) error {
	login, err := resumeInput(config)
	if err != nil {
		return &SessionError{Cause: err}
	}

	login.Request.ForceRefresh = config.forceRefresh
	login.Request.AllowUntrusted = config.allowUntrusted

	path := login.Output
	if config.saveSession != "" {
		path = config.saveSession
	}

	err = preserveReferenceFiles(path, login.SourceFiles)
	if err != nil {
		return &SessionError{Cause: err}
	}

	result, err := client.ResumeSession(ctx, login.Request)
	if err != nil {
		return fmt.Errorf("resume authentication: %w", err)
	}

	err = saveNativeSession(ctx, path, result)
	if err != nil {
		return &SessionError{Cause: err}
	}

	return writeResult(output, resumeSummary{SessionFile: path, Trusted: result.TrustedSession,
		RequiresTwoFactor: result.RequiresTwoFactor, RequiresTwoStep: result.RequiresTwoStep})
}

func resumeInput(config options) (*savedlogin.Login, error) {
	if config.referenceState != "" {
		login, err := savedlogin.Load(config.referenceState)
		if err != nil {
			return nil, fmt.Errorf("load reference session: %w", err)
		}

		return login, nil
	}

	data, err := os.ReadFile(filepath.Clean(config.session))
	if err != nil {
		return nil, fmt.Errorf("read native session: %w", err)
	}

	var saved icloud.ResumeSessionResult

	err = json.Unmarshal(data, &saved)
	if err != nil {
		return nil, fmt.Errorf("decode native session: %w", err)
	}

	if saved.Auth.SessionToken == nil {
		return nil, errNativeSession
	}

	var request icloud.ResumeSessionRequest

	request.Auth = icloud.SavedSessionCredentials{ClientID: saved.Auth.ClientID, SessionToken: *saved.Auth.SessionToken,
		SetupServiceURL: saved.Auth.SetupServiceURL, Headers: saved.Auth.Headers, Cookies: saved.Auth.Cookies,
		ChinaMainland: saved.Auth.ChinaMainland}
	request.AccountCountryCode = saved.AccountCountryCode
	request.TrustToken = saved.TrustToken

	return &savedlogin.Login{Request: request, Output: config.session, SourceFiles: nil}, nil
}

func saveNativeSession(ctx context.Context, path string, result *icloud.ResumeSessionResult) error {
	err := ctx.Err()
	if err != nil {
		return fmt.Errorf("private session canceled: %w", err)
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode private session: %w", err)
	}

	path = filepath.Clean(path)

	err = os.MkdirAll(filepath.Dir(path), privateSessionDirectoryMode)
	if err != nil {
		return fmt.Errorf("create private session directory: %w", err)
	}

	file, err := os.CreateTemp(filepath.Dir(path), ".go-icloud-session-*")
	if err != nil {
		return fmt.Errorf("create private session file: %w", err)
	}

	temporary := file.Name()

	err = writePrivateSession(ctx, file, data)
	if err != nil {
		return cleanupSessionFailure(temporary, fmt.Errorf("write private session: %w", err))
	}

	err = ctx.Err()
	if err != nil {
		return cleanupSessionFailure(temporary, fmt.Errorf("private session canceled: %w", err))
	}

	err = os.Rename(temporary, path)
	if err != nil {
		return cleanupSessionFailure(temporary, fmt.Errorf("replace private session: %w", err))
	}

	return nil
}

func writePrivateSession(ctx context.Context, file *os.File, data []byte) error {
	temporary := file.Name()

	var err error

	err = protectPrivateFile(ctx, temporary)
	if err == nil {
		err = ctx.Err()
	}

	if err == nil {
		_, err = file.Write(data)
	}

	if err == nil {
		err = file.Sync()
	}

	return errors.Join(err, file.Close())
}

func cleanupSessionFailure(path string, cause error) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return errors.Join(cause, fmt.Errorf("remove private temporary session: %w", err))
	}

	return cause
}
