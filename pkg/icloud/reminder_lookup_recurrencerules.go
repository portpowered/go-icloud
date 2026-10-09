package icloud

import (
	"context"
	"github.com/portpowered/go-icloud/internal/protocol"
)

// ListReminderRecurrenceRules reads ordered related records, retaining duplicates and response evidence.
func (sdk *SDK) ListReminderRecurrenceRules(ctx context.Context,
	request ListReminderRecurrenceRulesRequest) (*ListReminderRecurrenceRulesResult, error) {
	read, err := sdk.relatedRead(request.Auth, "ListReminderRecurrenceRules")
	if err != nil {
		return nil, err
	}

	records, err := read.records(ctx, request.IDs,
		protocol.RemindersRecurrenceRuleIDPrefixValue, protocol.RemindersRecurrenceRuleRecordTypeValue)
	if err != nil {
		return nil, read.failure(err)
	}

	result := &ListReminderRecurrenceRulesResult{Items: []ReminderRecurrenceRule{}, Responses: nil}

	for _, record := range records {
		value, projectErr := projectReminderRecurrence(record)
		if projectErr != nil {
			return nil, read.failure(projectErr)
		}

		result.Items = append(result.Items, value)
	}

	result.Responses = read.metadata()

	return result, nil
}
