package command

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/icloud"
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
	case "photo-album-create", "photo-album-rename", "photo-album-delete":
		return true
	case "photo-album-add", "photo-favorite", "photo-delete":
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
	case "photo-album-create", "photo-album-rename", "photo-album-delete":
		return runAlbumWrite(ctx, client, auth, operation, path, resultPath)
	case "photo-album-add", "photo-favorite", "photo-delete":
		return runAssetWrite(ctx, client, auth, operation, path, resultPath)
	default:
		return nil, errCommand
	}
}
