package command

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func executeService(ctx context.Context, client icloud.Client, auth icloud.AuthContext, config options) (any, error) {
	if fileWriteCommand(config.operation) {
		return runFileWrite(ctx, client, auth, config.operation, config.requestFile, config.contentFile, config.saveResult)
	}
	if writeCommand(config.operation) {
		return runWrite(ctx, client, auth, config.operation, config.requestFile, config.saveResult)
	}
	if typedReadCommand(config.operation) && (config.requestFile != "" || !photoFlagCommand(config.operation)) {
		return runTypedRead(ctx, client, auth, config.operation, config.requestFile, config.saveResult)
	}
	return read(ctx, client, auth, config)
}

func photoFlagCommand(operation string) bool {
	switch operation {
	case "photos-status", "photo-albums", "photo-count", "photo-assets", "photo", "photo-download":
		return true
	default:
		return false
	}
}
