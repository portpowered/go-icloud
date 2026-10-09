package icloud

import (
	"context"
	"github.com/portpowered/go-icloud/internal/protocol"
)

// ListReminderTags reads ordered related records, retaining duplicates and response evidence.
func (sdk *SDK) ListReminderTags(ctx context.Context,
	request ListReminderTagsRequest) (*ListReminderTagsResult, error) {
	read, err := sdk.relatedRead(request.Auth, "ListReminderTags")
	if err != nil {
		return nil, err
	}

	records, err := read.records(ctx, request.IDs,
		protocol.RemindersHashtagIDPrefixValue, protocol.RemindersHashtagRecordTypeValue)
	if err != nil {
		return nil, read.failure(err)
	}

	result := &ListReminderTagsResult{Items: []ReminderHashtag{}, Responses: nil}

	for _, record := range records {
		value, projectErr := projectReminderHashtag(record)
		if projectErr != nil {
			return nil, read.failure(projectErr)
		}

		result.Items = append(result.Items, value)
	}

	result.Responses = read.metadata()

	return result, nil
}
