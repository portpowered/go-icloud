package command

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func runAttachmentWrite(ctx context.Context, client icloud.Client,
	auth icloud.AuthContext, operation, path, resultPath string,
) (any, error) {
	switch operation {
	case writeAttachmentCreate:
		return invokeWrite(ctx, path, resultPath,
			func(input icloud.CreateReminderURLAttachmentRequest) (*icloud.ReminderAttachmentMutationResult, error) {
				input.Auth = auth

				return client.CreateReminderURLAttachment(ctx, input)
			})
	case writeAttachmentUpdate:
		return invokeWrite(ctx, path, resultPath,
			func(input icloud.UpdateReminderAttachmentRequest) (*icloud.UpdateReminderAttachmentResult, error) {
				input.Auth = auth

				return client.UpdateReminderAttachment(ctx, input)
			})
	case writeAttachmentDelete:
		return invokeWrite(ctx, path, resultPath,
			func(input icloud.DeleteReminderAttachmentRequest) (*icloud.ReminderAttachmentMutationResult, error) {
				input.Auth = auth

				return client.DeleteReminderAttachment(ctx, input)
			})
	default:
		return nil, errCommand
	}
}
