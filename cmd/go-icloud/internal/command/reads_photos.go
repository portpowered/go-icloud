package command

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func typedReadCommand(operation string) bool {
	switch operation {
	case "photo-libraries", "photo-cursor", "photo-changes", "photo-library-changes", "photos-recently-added",
		"shared-photo-albums", "shared-photo-count", "shared-photos", "shared-photo", "shared-photo-download", "photo-upload-status":
		return true
	default:
		return false
	}
}

func runTypedRead(ctx context.Context, client icloud.Client, auth icloud.AuthContext,
	operation, requestPath, resultPath string,
) (any, error) {
	switch operation {
	case "photo-libraries":
		return invokeWrite(ctx, requestPath, resultPath, func(input icloud.ListPhotoLibrariesRequest) (*icloud.ListPhotoLibrariesResult, error) {
			input.Auth = auth
			return client.ListPhotoLibraries(ctx, input)
		})
	case "photo-cursor":
		return invokeWrite(ctx, requestPath, resultPath, func(input icloud.GetPhotosCursorRequest) (*icloud.GetPhotosCursorResult, error) {
			input.Auth = auth
			return client.GetPhotosCursor(ctx, input)
		})
	case "photo-changes":
		return invokeWrite(ctx, requestPath, resultPath, func(input icloud.GetPhotoChangesRequest) (*icloud.GetPhotoChangesResult, error) {
			input.Auth = auth
			return client.GetPhotoChanges(ctx, input)
		})
	case "photo-library-changes":
		return invokeWrite(ctx, requestPath, resultPath, func(input icloud.GetPhotoLibraryChangesRequest) (*icloud.GetPhotoLibraryChangesResult, error) {
			input.Auth = auth
			return client.GetPhotoLibraryChanges(ctx, input)
		})
	case "photos-recently-added":
		return invokeWrite(ctx, requestPath, resultPath, func(input icloud.ListRecentlyAddedPhotosRequest) (*icloud.ListRecentlyAddedPhotosResult, error) {
			input.Auth = auth
			return client.ListRecentlyAddedPhotos(ctx, input)
		})
	case "photo-upload-status":
		return invokeWrite(ctx, requestPath, resultPath, func(input icloud.GetPhotoUploadStatusRequest) (*icloud.GetPhotoUploadStatusResult, error) {
			input.Auth = auth
			return client.GetPhotoUploadStatus(ctx, input)
		})
	default:
		return runSharedPhotoRead(ctx, client, auth, operation, requestPath, resultPath)
	}
}
