package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func photoVisitCommand(operation string) bool {
	return operation == photoVisitCommandName || operation == photosRecentVisitCommand
}

func runPhotoVisit(ctx context.Context, client icloud.Client, auth icloud.AuthContext,
	operation, requestPath, resultPath string, output io.Writer,
) error {
	visitor := photoConsoleVisitor(ctx, output)

	var result any

	var err error

	switch operation {
	case photoVisitCommandName:
		result, err = invokeWrite(ctx, requestPath, resultPath,
			func(input icloud.ListPhotoAssetsRequest) (*icloud.ListPhotoAssetsResult, error) {
				input.Auth = auth

				return client.VisitPhotoAssets(ctx, input, visitor)
			})
	case photosRecentVisitCommand:
		result, err = invokeWrite(ctx, requestPath, resultPath,
			func(input icloud.ListRecentlyAddedPhotosRequest) (*icloud.ListRecentlyAddedPhotosResult, error) {
				input.Auth = auth

				return client.VisitRecentlyAddedPhotos(ctx, input, visitor)
			})
	default:
		return errCommand
	}

	if err != nil {
		return err
	}

	err = json.NewEncoder(output).Encode(result)
	if err != nil {
		return fmt.Errorf("write photo stream summary: %w", err)
	}

	return nil
}

func photoConsoleVisitor(ctx context.Context, output io.Writer) icloud.PhotoVisitor {
	return func(event icloud.PhotoVisitEvent) (bool, error) {
		err := ctx.Err()
		if err != nil {
			return false, fmt.Errorf("visit photo stream: %w", err)
		}

		value, err := safeWriteResult(event)
		if err != nil {
			return false, err
		}

		err = json.NewEncoder(output).Encode(value)
		if err != nil {
			return false, fmt.Errorf("write photo stream item: %w", err)
		}

		return true, nil
	}
}
