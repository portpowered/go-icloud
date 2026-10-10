package command

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func runReminderWrite(ctx context.Context, client icloud.Client,
	auth icloud.AuthContext, operation, path, resultPath string,
) (any, error) {
	switch operation {
	case writeReminderCreate:
		return invokeWrite(ctx, path, resultPath,
			func(input icloud.CreateReminderRequest) (*icloud.ReminderMutationResult, error) {
				input.Auth = auth

				return client.CreateReminder(ctx, input)
			})
	case writeReminderUpdate:
		return invokeWrite(ctx, path, resultPath,
			func(input icloud.UpdateReminderRequest) (*icloud.ReminderMutationResult, error) {
				input.Auth = auth

				return client.UpdateReminder(ctx, input)
			})
	case writeReminderDelete:
		return invokeWrite(ctx, path, resultPath,
			func(input icloud.DeleteReminderRequest) (*icloud.DeleteReminderResult, error) {
				input.Auth = auth

				return client.DeleteReminder(ctx, input)
			})
	default:
		return nil, errCommand
	}
}
