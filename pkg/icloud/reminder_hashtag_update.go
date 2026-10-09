package icloud

import (
	"context"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const updateReminderHashtagOperation = "UpdateReminderHashtag"

// UpdateReminderHashtag renames a hashtag and returns its refreshed revision without changing the input.
func (sdk *SDK) UpdateReminderHashtag(ctx context.Context,
	request UpdateReminderHashtagRequest,
) (*ReminderHashtagMutationResult, error) {
	auth, err := reminderWriteContext(ctx, request.Auth, updateReminderHashtagOperation)
	if err != nil {
		return nil, err
	}

	child := copyReminderHashtag(request.Hashtag)
	name := reminderRelatedRecordName(child.ID, protocol.RemindersHashtagIDPrefixValue)
	fields := cloudkit.ReminderHashtagUpdateFields{Name: reminderHashtagName(request.Name), Reminder: nil}

	if child.ReminderID != "" {
		parent := reminderWriteReference(reminderRecordName(child.ReminderID))
		fields.Reminder = &parent
	}

	input := cloudkit.ReminderHashtagUpdateRequest{ZoneID: reminderWriteZone(),
		Operations: []cloudkit.ReminderHashtagUpdateOperation{{OperationType: cloudkit.ReminderHashtagUpdateOperationType,
			Record: cloudkit.ReminderHashtagUpdateRecord{RecordName: name,
				RecordType: cloudkit.ReminderHashtagUpdateRecordTypeHashtag, Fields: fields, PluginFields: map[string]any{},
				RecordChangeTag: reminderRevisionPointer(child.RecordChangeTag)}}}}

	response, err := sdk.web.UpdateReminderHashtag(ctx, auth, input)
	if err != nil {
		return nil, adaptFailure(updateReminderHashtagOperation, err)
	}

	tags, err := reminderAcknowledgements(updateReminderHashtagOperation, response)
	if err != nil {
		return nil, err
	}

	child.Name = request.Name
	child.RecordChangeTag = reminderAcknowledgedTag(child.RecordChangeTag, name, tags)

	return &ReminderHashtagMutationResult{Hashtag: child,
		Responses: []ResponseMetadata{publicMetadata(response.Metadata)}}, nil
}
