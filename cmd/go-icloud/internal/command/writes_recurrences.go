package command

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func runRecurrenceWrite(ctx context.Context, client icloud.Client,
	auth icloud.AuthContext, operation, path, resultPath string,
) (any, error) {
	switch operation {
	case writeRecurrenceCreate:
		return invokeWrite(ctx, path, resultPath,
			func(input icloud.CreateReminderRecurrenceRuleRequest) (*icloud.ReminderRecurrenceRuleRelationResult, error) {
				input.Auth = auth

				return client.CreateReminderRecurrenceRule(ctx, input)
			})
	case writeRecurrenceUpdate:
		return invokeWrite(ctx, path, resultPath,
			func(input icloud.UpdateReminderRecurrenceRuleRequest) (*icloud.ReminderRecurrenceRuleMutationResult, error) {
				input.Auth = auth

				return client.UpdateReminderRecurrenceRule(ctx, input)
			})
	case writeRecurrenceDelete:
		return invokeWrite(ctx, path, resultPath,
			func(input icloud.DeleteReminderRecurrenceRuleRequest) (*icloud.ReminderRecurrenceRuleRelationResult, error) {
				input.Auth = auth

				return client.DeleteReminderRecurrenceRule(ctx, input)
			})
	default:
		return nil, errCommand
	}
}
