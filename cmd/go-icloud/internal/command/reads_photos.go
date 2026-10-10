package command

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func typedReadCommand(operation string) bool {
	switch operation {
	case photoLibrariesCommand, photoCursorCommand, photoChangesCommand, photoLibraryChangesCommand, photosRecentCommand,
		sharedPhotoAlbumsCommand, sharedPhotoCountCommand, sharedPhotosCommand,
		sharedPhotoCommand, sharedPhotoDownloadCommand, photoUploadStatusCommand:
		return true
	case photosStatusCommand, photoAlbumsCommand, photoCountCommand,
		photoAssetsCommand, photoCommand, photoDownloadCommand:
		return true
	default:
		return false
	}
}

func runTypedRead(ctx context.Context, client icloud.Client, auth icloud.AuthContext,
	operation, requestPath, resultPath string,
) (any, error) {
	switch operation {
	case photoLibrariesCommand:
		return invokeWrite(ctx, requestPath, resultPath,
			func(input icloud.ListPhotoLibrariesRequest) (*icloud.ListPhotoLibrariesResult, error) {
				input.Auth = auth
				return client.ListPhotoLibraries(ctx, input)
			})
	case photoCursorCommand:
		return invokeWrite(ctx, requestPath, resultPath,
			func(input icloud.GetPhotosCursorRequest) (*icloud.GetPhotosCursorResult, error) {
				input.Auth = auth
				return client.GetPhotosCursor(ctx, input)
			})
	case photoChangesCommand:
		return invokeWrite(ctx, requestPath, resultPath,
			func(input icloud.GetPhotoChangesRequest) (*icloud.GetPhotoChangesResult, error) {
				input.Auth = auth
				return client.GetPhotoChanges(ctx, input)
			})
	case photoLibraryChangesCommand:
		return invokeWrite(ctx, requestPath, resultPath,
			func(input icloud.GetPhotoLibraryChangesRequest) (*icloud.GetPhotoLibraryChangesResult, error) {
				input.Auth = auth
				return client.GetPhotoLibraryChanges(ctx, input)
			})
	case photosRecentCommand:
		return invokeWrite(ctx, requestPath, resultPath,
			func(input icloud.ListRecentlyAddedPhotosRequest) (*icloud.ListRecentlyAddedPhotosResult, error) {
				input.Auth = auth
				return client.ListRecentlyAddedPhotos(ctx, input)
			})
	case photoUploadStatusCommand:
		return invokeWrite(ctx, requestPath, resultPath,
			func(input icloud.GetPhotoUploadStatusRequest) (*icloud.GetPhotoUploadStatusResult, error) {
				input.Auth = auth
				return client.GetPhotoUploadStatus(ctx, input)
			})
	case photosStatusCommand:
		return invokeWrite(ctx, requestPath, resultPath,
			func(input icloud.GetPhotosStatusRequest) (*icloud.GetPhotosStatusResult, error) {
				input.Auth = auth
				return client.GetPhotosStatus(ctx, input)
			})
	case photoAlbumsCommand:
		return invokeWrite(ctx, requestPath, resultPath,
			func(input icloud.ListPhotoAlbumsRequest) (*icloud.ListPhotoAlbumsResult, error) {
				input.Auth = auth
				return client.ListPhotoAlbums(ctx, input)
			})
	case photoCountCommand:
		return invokeWrite(ctx, requestPath, resultPath,
			func(input icloud.GetPhotoAlbumCountRequest) (*icloud.GetPhotoAlbumCountResult, error) {
				input.Auth = auth
				return client.GetPhotoAlbumCount(ctx, input)
			})
	case photoAssetsCommand:
		return invokeWrite(ctx, requestPath, resultPath,
			func(input icloud.ListPhotoAssetsRequest) (*icloud.ListPhotoAssetsResult, error) {
				input.Auth = auth
				return client.ListPhotoAssets(ctx, input)
			})
	case photoCommand:
		return invokeWrite(ctx, requestPath, resultPath,
			func(input icloud.GetPhotoRequest) (*icloud.GetPhotoResult, error) {
				input.Auth = auth
				return client.GetPhoto(ctx, input)
			})
	case photoDownloadCommand:
		return invokeWrite(ctx, requestPath, resultPath,
			func(input icloud.DownloadPhotoRequest) (*icloud.DownloadPhotoResult, error) {
				input.Auth = auth
				return client.DownloadPhoto(ctx, input)
			})
	default:
		return runSharedPhotoRead(ctx, client, auth, operation, requestPath, resultPath)
	}
}
