package command

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func runAssetWrite(ctx context.Context, client icloud.Client,
	auth icloud.AuthContext, operation, path, resultPath string,
) (any, error) {
	switch operation {
	case "photo-album-add":
		return invokeWrite(ctx, path, resultPath, func(input icloud.AddPhotoToAlbumRequest) (*icloud.PhotoAlbumRelationResult, error) {
			input.Auth = auth

			return client.AddPhotoToAlbum(ctx, input)
		})
	case "photo-favorite":
		return invokeWrite(ctx, path, resultPath, func(input icloud.SetPhotoFavoriteRequest) (*icloud.PhotoMutationResult, error) {
			input.Auth = auth

			return client.SetPhotoFavorite(ctx, input)
		})
	case "photo-delete":
		return invokeWrite(ctx, path, resultPath, func(input icloud.DeletePhotoRequest) (*icloud.PhotoDeletionResult, error) {
			input.Auth = auth

			return client.DeletePhoto(ctx, input)
		})
	default:
		return nil, errCommand
	}
}
