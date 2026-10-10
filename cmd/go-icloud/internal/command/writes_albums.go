package command

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func runAlbumWrite(ctx context.Context, client icloud.Client,
	auth icloud.AuthContext, operation, path, resultPath string,
) (any, error) {
	switch operation {
	case writePhotoAlbumCreate:
		return invokeWrite(ctx, path, resultPath,
			func(input icloud.CreatePhotoAlbumRequest) (*icloud.PhotoAlbumMutationResult, error) {
				input.Auth = auth

				return client.CreatePhotoAlbum(ctx, input)
			})
	case writePhotoAlbumRename:
		return invokeWrite(ctx, path, resultPath,
			func(input icloud.RenamePhotoAlbumRequest) (*icloud.PhotoAlbumMutationResult, error) {
				input.Auth = auth

				return client.RenamePhotoAlbum(ctx, input)
			})
	case writePhotoAlbumDelete:
		return invokeWrite(ctx, path, resultPath,
			func(input icloud.DeletePhotoAlbumRequest) (*icloud.PhotoDeletionResult, error) {
				input.Auth = auth

				return client.DeletePhotoAlbum(ctx, input)
			})
	default:
		return nil, errCommand
	}
}
