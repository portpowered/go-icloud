package command

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func runAlbumRead(ctx context.Context, client icloud.Client, auth icloud.AuthContext,
	operation, requestPath, resultPath string,
) (any, error) {
	switch operation {
	case "photos-status":
		return invokeWrite(ctx, requestPath, resultPath, func(input icloud.GetPhotosStatusRequest) (*icloud.GetPhotosStatusResult, error) {
			input.Auth = auth
			return client.GetPhotosStatus(ctx, input)
		})
	case "photo-albums":
		return invokeWrite(ctx, requestPath, resultPath, func(input icloud.ListPhotoAlbumsRequest) (*icloud.ListPhotoAlbumsResult, error) {
			input.Auth = auth
			return client.ListPhotoAlbums(ctx, input)
		})
	case "photo-count":
		return invokeWrite(ctx, requestPath, resultPath, func(input icloud.GetPhotoAlbumCountRequest) (*icloud.GetPhotoAlbumCountResult, error) {
			input.Auth = auth
			return client.GetPhotoAlbumCount(ctx, input)
		})
	case "photo-assets":
		return invokeWrite(ctx, requestPath, resultPath, func(input icloud.ListPhotoAssetsRequest) (*icloud.ListPhotoAssetsResult, error) {
			input.Auth = auth
			return client.ListPhotoAssets(ctx, input)
		})
	case "photo":
		return invokeWrite(ctx, requestPath, resultPath, func(input icloud.GetPhotoRequest) (*icloud.GetPhotoResult, error) {
			input.Auth = auth
			return client.GetPhoto(ctx, input)
		})
	case "photo-download":
		return invokeWrite(ctx, requestPath, resultPath, func(input icloud.DownloadPhotoRequest) (*icloud.DownloadPhotoResult, error) {
			input.Auth = auth
			return client.DownloadPhoto(ctx, input)
		})
	default:
		return runSharedPhotoRead(ctx, client, auth, operation, requestPath, resultPath)
	}
}
