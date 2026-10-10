package command

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

const (
	writePhotoAlbumCreate    = "photo-album-create"
	writePhotoAlbumRename    = "photo-album-rename"
	writePhotoAlbumDelete    = "photo-album-delete"
	writePhotoAlbumAdd       = "photo-album-add"
	writePhotoFavorite       = "photo-favorite"
	writePhotoDelete         = "photo-delete"
	writePhotoUpload         = "photo-upload"
	writePhotoUploadRegister = "photo-upload-register"
)

func writeCommand(operation string) bool {
	if fileWriteCommand(operation) || phaseWriteCommand(operation) {
		return true
	}

	switch operation {
	case "reminder-create", "reminder-update", "reminder-delete":
		return true
	case "reminder-hashtag-create", "reminder-hashtag-update", "reminder-hashtag-delete":
		return true
	case "reminder-recurrence-create", "reminder-recurrence-update", "reminder-recurrence-delete":
		return true
	case "reminder-attachment-create", "reminder-attachment-update", "reminder-attachment-delete":
		return true
	case "reminder-location-add":
		return true
	case writePhotoAlbumCreate, writePhotoAlbumRename, writePhotoAlbumDelete:
		return true
	case writePhotoAlbumAdd, writePhotoFavorite, writePhotoDelete:
		return true
	default:
		return false
	}
}

func runWrite(ctx context.Context, client icloud.Client,
	auth icloud.AuthContext, operation, path, resultPath string,
) (any, error) {
	if phaseWriteCommand(operation) {
		return runUploadWrite(ctx, client, auth, operation, path, resultPath)
	}

	switch operation {
	case "reminder-create", "reminder-update", "reminder-delete":
		return runReminderWrite(ctx, client, auth, operation, path, resultPath)
	case "reminder-hashtag-create", "reminder-hashtag-update", "reminder-hashtag-delete":
		return runHashtagWrite(ctx, client, auth, operation, path, resultPath)
	case "reminder-recurrence-create", "reminder-recurrence-update", "reminder-recurrence-delete":
		return runRecurrenceWrite(ctx, client, auth, operation, path, resultPath)
	case "reminder-attachment-create", "reminder-attachment-update", "reminder-attachment-delete":
		return runAttachmentWrite(ctx, client, auth, operation, path, resultPath)
	case "reminder-location-add":
		return runLocationWrite(ctx, client, auth, operation, path, resultPath)
	case writePhotoAlbumCreate, writePhotoAlbumRename, writePhotoAlbumDelete:
		return runAlbumWrite(ctx, client, auth, operation, path, resultPath)
	case writePhotoAlbumAdd, writePhotoFavorite, writePhotoDelete:
		return runAssetWrite(ctx, client, auth, operation, path, resultPath)
	default:
		return nil, errCommand
	}
}
