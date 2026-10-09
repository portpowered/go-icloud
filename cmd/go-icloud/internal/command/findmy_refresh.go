package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func recoverFindMyRead(ctx context.Context, client icloud.Client, auth icloud.AuthContext,
	config options, original error,
) (any, error) {
	var failure *icloud.ClientError
	if config.operation != "findmy" || !errors.As(original, &failure) || failure.Kind() != icloud.AuthenticationRequired {
		return nil, original
	}

	var native options

	native.session = config.session

	login, err := resumeInput(native)
	if err != nil {
		return nil, original
	}

	login.Request.ForceRefresh = true

	login.Request.ResponseUpdates = append(failure.PriorResponses(), icloud.ResponseMetadata{
		StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders(), CookieScopeURL: failure.CookieScopeURL()})

	refreshed, err := client.ResumeSession(ctx, login.Request)
	if err != nil {
		return nil, fmt.Errorf("refresh Find My authentication: %w", err)
	}

	err = saveNativeSession(ctx, config.session, refreshed)
	if err != nil {
		return nil, &SessionError{Cause: err}
	}

	return retryFindMyRead(ctx, client, auth, config, refreshed)
}

func retryFindMyRead(ctx context.Context, client icloud.Client, original icloud.AuthContext,
	config options, refreshed *icloud.ResumeSessionResult,
) (any, error) {
	// Source binds its manager to the original origin across authentication discovery.
	readAuth := refreshed.Auth
	readAuth.FindMyServiceURL = original.FindMyServiceURL

	session, err := client.OpenFindMySession(ctx, icloud.OpenFindMySessionRequest{Auth: readAuth,
		IncludeFamily: config.family},
		icloud.WithFindMyMonitorInterval(0))
	if err != nil {
		return nil, fmt.Errorf("retry Find My discovery: %w", err)
	}

	snapshot, snapshotErr := session.Snapshot()
	finalAuth := session.Authentication()
	refreshed.Responses = append(refreshed.Responses, session.LastResponses()...)
	closeErr := session.Close()

	err = errors.Join(snapshotErr, closeErr)
	if err != nil {
		return nil, fmt.Errorf("finish refreshed Find My session: %w", err)
	}

	finalAuth.FindMyServiceURL = refreshed.Auth.FindMyServiceURL
	refreshed.Auth = finalAuth

	err = saveNativeSession(ctx, config.session, refreshed)
	if err != nil {
		return nil, &SessionError{Cause: err}
	}

	return snapshot.Devices, nil
}
