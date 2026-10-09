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
		var failure *icloud.ClientError

		if errors.As(err, &failure) && failure.StatusCode() != 0 {
			responses := append(failure.PriorResponses(), icloud.ResponseMetadata{StatusCode: failure.StatusCode(),
				Headers: failure.ResponseHeaders(), CookieScopeURL: failure.CookieScopeURL()})
			err = errors.Join(err, persistFindMyResponses(ctx, client, config.session, refreshed, responses))
		}

		return nil, fmt.Errorf("retry Find My discovery: %w", err)
	}

	snapshot, snapshotErr := session.Snapshot()
	responses := session.LastResponses()
	closeErr := session.Close()

	err = errors.Join(snapshotErr, closeErr, persistFindMyResponses(ctx, client, config.session, refreshed, responses))
	if err != nil {
		return nil, fmt.Errorf("finish refreshed Find My session: %w", err)
	}

	return snapshot.Devices, nil
}

func persistFindMyResponses(ctx context.Context, client icloud.Client, path string,
	saved *icloud.ResumeSessionResult, responses []icloud.ResponseMetadata,
) error {
	updated, err := client.ApplySessionResponses(ctx, icloud.ApplySessionResponsesRequest{
		Session: *saved, Responses: responses})
	if err != nil {
		return fmt.Errorf("apply Find My response updates: %w", err)
	}

	err = saveNativeSession(ctx, path, &updated.Session)
	if err != nil {
		return &SessionError{Cause: err}
	}

	return nil
}
