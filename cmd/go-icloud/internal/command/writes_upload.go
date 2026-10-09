package command

import (
	"context"
	"errors"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

var errWriteReceipt = errors.New("provide --save-result for composable photo upload commands")

func phaseWriteCommand(operation string) bool {
	return operation == "photo-upload-reserve" || operation == "photo-upload-send" || operation == "photo-upload-register"
}

func runUploadWrite(ctx context.Context, client icloud.Client,
	auth icloud.AuthContext, operation, path, resultPath string,
) (any, error) {
	if resultPath == "" {
		return nil, errWriteReceipt
	}
	switch operation {
	case "photo-upload-reserve":
		return invokeWrite(ctx, path, resultPath,
			func(input icloud.ReservePhotoUploadsRequest) (*icloud.ReservePhotoUploadsResult, error) {
				input.Auth = auth

				return client.ReservePhotoUploads(ctx, input)
			})
	case "photo-upload-register":
		return invokeWrite(ctx, path, resultPath,
			func(input icloud.RegisterPhotoUploadsRequest) (*icloud.RegisterPhotoUploadsResult, error) {
				input.Auth = auth

				return client.RegisterPhotoUploads(ctx, input)
			})
	default:
		return nil, errCommand
	}
}
