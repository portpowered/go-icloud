package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/pkg/photosync"
)

const photosCredentialSaveTimeout = 5 * time.Second

func (progress *photosSyncProgress) bind(ctx context.Context, library string) error {
	libraries := []icloud.PhotoLibrary{}
	if library != string(photosync.RootLibrary) {
		result, err := progress.client.ListPhotoLibraries(ctx, icloud.ListPhotoLibrariesRequest{Auth: progress.session.Auth})
		if err != nil {
			return progress.recordFailure(ctx, err)
		}

		err = progress.apply(ctx, result.Responses)
		if err != nil {
			return err
		}

		libraries = result.Libraries
	}

	source, err := photosync.NewSDKSource(ctx, progress.client, progress.session, libraries)
	if err != nil {
		return fmt.Errorf("bind Photos session: %w", err)
	}

	progress.source = source

	return nil
}

func (progress *photosSyncProgress) apply(ctx context.Context, responses []icloud.ResponseMetadata) error {
	stateContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), photosCredentialSaveTimeout)
	defer cancel()

	result, err := progress.client.ApplySessionResponses(stateContext,
		icloud.ApplySessionResponsesRequest{Session: progress.session, Responses: responses})
	if err != nil {
		return fmt.Errorf("apply Photos response credentials: %w", err)
	}

	progress.session = result.Session

	return nil
}

func (progress *photosSyncProgress) recordFailure(ctx context.Context, cause error) error {
	var failure *icloud.ClientError
	if !errors.As(cause, &failure) {
		return cause
	}

	responses := failure.PriorResponses()
	if failure.StatusCode() != 0 {
		responses = append(responses, icloud.ResponseMetadata{StatusCode: failure.StatusCode(),
			Headers: failure.ResponseHeaders(), CookieScopeURL: failure.CookieScopeURL()})
	}

	return errors.Join(cause, progress.apply(ctx, responses))
}

func (progress *photosSyncProgress) persist(ctx context.Context) error {
	stateContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), photosCredentialSaveTimeout)
	defer cancel()
	if progress.source != nil {
		session, err := progress.source.Snapshot(stateContext)
		if err != nil {
			return fmt.Errorf("snapshot Photos credentials: %w", err)
		}

		progress.session = *session
	}

	progress.state.Auth, progress.state.TrustToken = progress.session.Auth, progress.session.TrustToken
	progress.state.AccountCountryCode, progress.state.AccountData =
		progress.session.AccountCountryCode, progress.session.AccountData

	return saveNativeAuthentication(stateContext, progress.path, progress.state)
}
