package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/photosynccommand"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/pkg/photosync"
)

const (
	photosSyncOperation  = "photos-sync"
	photosWatchOperation = "photos-watch"
)

var (
	errPhotosSyncRequest = errors.New("provide --request with the generated Photos sync or watch request")
	errPhotosSyncClient  = errors.New("photos synchronization SDK capabilities are unavailable")
)

type photosSyncClient interface {
	photosync.SDKClient
	ListPhotoLibraries(ctx context.Context,
		request icloud.ListPhotoLibrariesRequest,
	) (*icloud.ListPhotoLibrariesResult, error)
}

type photosSyncProgress struct {
	client  photosSyncClient
	state   icloud.NativeAuthState
	session icloud.ResumeSessionResult
	source  *photosync.SDKSource
	path    string
}

func photosSyncCommand(operation string) bool {
	return operation == photosSyncOperation || operation == photosWatchOperation
}

func runPhotosSync(ctx context.Context, client icloud.Client, config options, output io.Writer) (failure error) {
	request, watch, err := readPhotosSyncInput(ctx, config)
	if err != nil {
		return err
	}

	sdk, supported := client.(photosSyncClient)
	if !supported {
		return errPhotosSyncClient
	}

	state, err := loadNativeAuthentication(config.session)
	if err != nil {
		return err
	}

	progress := newPhotosSyncProgress(sdk, state, config.session)
	defer func() { failure = errors.Join(failure, progress.persist(ctx)) }()

	request.Auth = state.Auth

	err = progress.bind(ctx, request.Options.Library)
	if err != nil {
		return err
	}

	engine, err := photosync.New(progress.source,
		photosync.Configuration{Files: photosync.OSFileSystem{}, Now: nil, Wait: nil})
	if err != nil {
		return fmt.Errorf("create Photos synchronization: %w", err)
	}

	if watch != nil {
		err = engine.Watch(ctx, *request, *watch, func(result photosync.Result) error {
			return emitPhotosSync(ctx, progress, config.saveResult, output, result)
		})
		if err != nil {
			return fmt.Errorf("watch Photos synchronization: %w", err)
		}

		return nil
	}

	result, err := engine.Run(ctx, *request)
	if err != nil {
		return fmt.Errorf("synchronize Photos: %w", err)
	}

	return emitPhotosSync(ctx, progress, config.saveResult, output, *result)
}

func readPhotosSyncInput(ctx context.Context, config options) (*photosync.Request, *photosync.WatchOptions, error) {
	if config.requestFile == "" {
		return nil, nil, errPhotosSyncRequest
	}

	if config.operation == photosWatchOperation {
		input, err := readWriteRequest[photosynccommand.PhotosWatchRequest](ctx, config.requestFile)
		if err != nil {
			return nil, nil, err
		}

		return &input.Request, &input.Watch, nil
	}

	input, err := readWriteRequest[photosync.Request](ctx, config.requestFile)
	if err != nil {
		return nil, nil, err
	}

	return input, nil, nil
}

func newPhotosSyncProgress(client photosSyncClient, state icloud.NativeAuthState, path string) *photosSyncProgress {
	session := new(icloud.ResumeSessionResult)
	session.Auth, session.TrustToken = state.Auth, state.TrustToken
	session.AccountCountryCode, session.AccountData = state.AccountCountryCode, state.AccountData
	session.RequiresTwoFactor, session.TrustedSession = state.RequiresMFA, !state.RequiresMFA
	session.Responses = []icloud.ResponseMetadata{}

	return &photosSyncProgress{client: client, state: state, session: *session, source: nil, path: path}
}

func emitPhotosSync(ctx context.Context, progress *photosSyncProgress, destination string,
	output io.Writer, result photosync.Result,
) error {
	err := progress.persist(ctx)
	if err != nil {
		return err
	}

	if destination != "" {
		data, encodeErr := json.Marshal(result)
		if encodeErr != nil {
			return fmt.Errorf("encode Photos result: %w", encodeErr)
		}

		err = savePrivateData(ctx, destination, data)
		if err != nil {
			return &SessionError{Cause: err}
		}
	}

	projected, err := safeWriteResult(result)
	if err != nil {
		return err
	}

	return writeResult(output, projected)
}
