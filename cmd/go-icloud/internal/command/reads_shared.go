package command

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func runSharedPhotoRead(ctx context.Context, client icloud.Client, auth icloud.AuthContext,
	operation, requestPath, resultPath string,
) (any, error) {
	switch operation {
	case sharedPhotoAlbumsCommand:
		return invokeWrite(ctx, requestPath, resultPath,
			func(input icloud.ListSharedPhotoAlbumsRequest) (*icloud.ListSharedPhotoAlbumsResult, error) {
				input.Auth = auth
				return client.ListSharedPhotoAlbums(ctx, input)
			})
	case sharedPhotoCountCommand:
		return invokeWrite(ctx, requestPath, resultPath,
			func(input icloud.CountSharedPhotosRequest) (*icloud.CountSharedPhotosResult, error) {
				input.Auth = auth
				return client.CountSharedPhotos(ctx, input)
			})
	case sharedPhotosCommand:
		return invokeWrite(ctx, requestPath, resultPath,
			func(input icloud.ListSharedPhotosRequest) (*icloud.ListSharedPhotosResult, error) {
				input.Auth = auth
				return client.ListSharedPhotos(ctx, input)
			})
	case sharedPhotoCommand:
		return invokeWrite(ctx, requestPath, resultPath,
			func(input icloud.GetSharedPhotoRequest) (*icloud.GetSharedPhotoResult, error) {
				input.Auth = auth
				return client.GetSharedPhoto(ctx, input)
			})
	case sharedPhotoDownloadCommand:
		return invokeWrite(ctx, requestPath, resultPath,
			func(input icloud.DownloadSharedPhotoRequest) (*icloud.DownloadSharedPhotoResult, error) {
				input.Auth = auth
				return client.DownloadSharedPhoto(ctx, input)
			})
	default:
		return nil, errCommand
	}
}
