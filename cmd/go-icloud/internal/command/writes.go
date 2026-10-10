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
	case writeReminderCreate, writeReminderUpdate, writeReminderDelete:
		return true
	case writeHashtagCreate, writeHashtagUpdate, writeHashtagDelete:
		return true
	case writeRecurrenceCreate, writeRecurrenceUpdate, writeRecurrenceDelete:
		return true
	case writeAttachmentCreate, writeAttachmentUpdate, writeAttachmentDelete:
		return true
	case writeLocationAdd:
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
	case writeReminderCreate, writeReminderUpdate, writeReminderDelete:
		return runReminderWrite(ctx, client, auth, operation, path, resultPath)
	case writeHashtagCreate, writeHashtagUpdate, writeHashtagDelete:
		return runHashtagWrite(ctx, client, auth, operation, path, resultPath)
	case writeRecurrenceCreate, writeRecurrenceUpdate, writeRecurrenceDelete:
		return runRecurrenceWrite(ctx, client, auth, operation, path, resultPath)
	case writeAttachmentCreate, writeAttachmentUpdate, writeAttachmentDelete:
		return runAttachmentWrite(ctx, client, auth, operation, path, resultPath)
	case writeLocationAdd:
		return runLocationWrite(ctx, client, auth, operation, path, resultPath)
	case writePhotoAlbumCreate, writePhotoAlbumRename, writePhotoAlbumDelete:
		return runAlbumWrite(ctx, client, auth, operation, path, resultPath)
	case writePhotoAlbumAdd, writePhotoFavorite, writePhotoDelete:
		return runAssetWrite(ctx, client, auth, operation, path, resultPath)
	default:
		return nil, errCommand
	}
}
