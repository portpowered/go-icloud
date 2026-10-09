package icloud

import (
	"context"
	"github.com/portpowered/go-icloud/internal/protocol"
)

// ListReminderAttachments reads ordered related records, retaining duplicates and response evidence.
func (sdk *SDK) ListReminderAttachments(ctx context.Context,
	request ListReminderAttachmentsRequest) (*ListReminderAttachmentsResult, error) {
	read, err := sdk.relatedRead(request.Auth, "ListReminderAttachments")
	if err != nil {
		return nil, err
	}

	records, err := read.records(ctx, request.IDs,
		protocol.RemindersAttachmentIDPrefixValue, protocol.RemindersAttachmentRecordTypeValue)
	if err != nil {
		return nil, read.failure(err)
	}

	result := &ListReminderAttachmentsResult{Items: []ReminderAttachment{}, Responses: nil}

	for _, record := range records {
		value, projectErr := projectReminderAttachment(record)
		if projectErr != nil {
			return nil, read.failure(projectErr)
		}

		if value != nil {
			result.Items = append(result.Items, *value)
		}
	}

	result.Responses = read.metadata()

	return result, nil
}
