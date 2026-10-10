package command

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func runLocationWrite(ctx context.Context, client icloud.Client,
	auth icloud.AuthContext, operation, path, resultPath string,
) (any, error) {
	switch operation {
	case "reminder-location-add":
		return invokeWrite(ctx, path, resultPath,
			func(input icloud.AddReminderLocationTriggerRequest) (*icloud.AddReminderLocationTriggerResult, error) {
				input.Auth = auth

				return client.AddReminderLocationTrigger(ctx, input)
			})
	default:
		return nil, errCommand
	}
}
