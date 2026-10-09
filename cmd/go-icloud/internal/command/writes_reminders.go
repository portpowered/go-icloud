package command

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func runReminderWrite(ctx context.Context, client icloud.Client,
	auth icloud.AuthContext, operation, path, resultPath string,
) (any, error) {
	switch operation {
	case "reminder-create":
		return invokeWrite(ctx, path, resultPath, func(input icloud.CreateReminderRequest) (*icloud.ReminderMutationResult, error) {
			input.Auth = auth

			return client.CreateReminder(ctx, input)
		})
	case "reminder-update":
		return invokeWrite(ctx, path, resultPath, func(input icloud.UpdateReminderRequest) (*icloud.ReminderMutationResult, error) {
			input.Auth = auth

			return client.UpdateReminder(ctx, input)
		})
	case "reminder-delete":
		return invokeWrite(ctx, path, resultPath, func(input icloud.DeleteReminderRequest) (*icloud.DeleteReminderResult, error) {
			input.Auth = auth

			return client.DeleteReminder(ctx, input)
		})
	default:
		return nil, errCommand
	}
}
