package command

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func runHashtagWrite(ctx context.Context, client icloud.Client,
	auth icloud.AuthContext, operation, path, resultPath string,
) (any, error) {
	switch operation {
	case "reminder-hashtag-create":
		return invokeWrite(ctx, path, resultPath, func(input icloud.CreateReminderHashtagRequest) (*icloud.ReminderHashtagRelationResult, error) {
			input.Auth = auth

			return client.CreateReminderHashtag(ctx, input)
		})
	case "reminder-hashtag-update":
		return invokeWrite(ctx, path, resultPath, func(input icloud.UpdateReminderHashtagRequest) (*icloud.ReminderHashtagMutationResult, error) {
			input.Auth = auth

			return client.UpdateReminderHashtag(ctx, input)
		})
	case "reminder-hashtag-delete":
		return invokeWrite(ctx, path, resultPath, func(input icloud.DeleteReminderHashtagRequest) (*icloud.ReminderHashtagRelationResult, error) {
			input.Auth = auth

			return client.DeleteReminderHashtag(ctx, input)
		})
	default:
		return nil, errCommand
	}
}
