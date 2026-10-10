package command

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

var errWriteFile = errors.New("provide --file with --request for photo-upload or photo-upload-file")

func fileWriteCommand(operation string) bool {
	return operation == writePhotoUpload || operation == "photo-upload-file" || operation == "photo-upload-send"
}

func runFileWrite(ctx context.Context, client icloud.Client,
	auth icloud.AuthContext, operation, requestPath, contentPath, resultPath string,
) (any, error) {
	if operation == "photo-upload-send" {
		return runUploadSend(ctx, client, auth, requestPath, contentPath, resultPath)
	}

	switch operation {
	case writePhotoUpload:
		return invokeFileWrite(ctx, requestPath, contentPath, resultPath,
			func(input icloud.UploadPhotoRequest, content io.ReadSeeker) (*icloud.UploadPhotoResult, error) {
				input.Auth = auth
				input.Content = content

				return client.UploadPhoto(ctx, input)
			})
	case "photo-upload-file":
		return invokeFileWrite(ctx, requestPath, contentPath, resultPath,
			func(input icloud.UploadPhotoFileRequest, content io.ReadSeeker) (*icloud.UploadPhotoFileResult, error) {
				input.Auth = auth
				input.Content = content

				return client.UploadPhotoFile(ctx, input)
			})
	default:
		return nil, errCommand
	}
}

func runUploadSend(ctx context.Context, client icloud.Client,
	auth icloud.AuthContext, requestPath, contentPath, resultPath string,
) (any, error) {
	if resultPath == "" {
		return nil, errWriteReceipt
	}

	return invokeFileWrite(ctx, requestPath, contentPath, resultPath,
		func(input icloud.SendPhotoUploadBytesRequest, content io.ReadSeeker) (*icloud.SendPhotoUploadBytesResult, error) {
			input.Auth = auth
			input.Content = content

			return client.SendPhotoUploadBytes(ctx, input)
		})
}

func invokeFileWrite[Request, Result any](ctx context.Context, requestPath, contentPath, resultPath string,
	invoke func(Request, io.ReadSeeker) (*Result, error),
) (any, error) {
	if contentPath == "" {
		return nil, errWriteFile
	}
	request, err := readWriteRequest[Request](ctx, requestPath)
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, &WriteRequestError{Cause: err}
	}

	content, err := openWriteFile(contentPath)
	if err != nil {
		return nil, &WriteRequestError{Cause: err}
	}

	result, writeErr := invoke(*request, content)
	closeErr := content.Close()
	if writeErr != nil {
		return nil, fmt.Errorf("SDK file write: %w", writeErr)
	}
	if closeErr != nil {
		return nil, &WriteRequestError{Cause: closeErr}
	}

	return finishWriteResult(ctx, result, resultPath)
}
