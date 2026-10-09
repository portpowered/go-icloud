package icloud

import (
	"time"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func (sdk *SDK) reminderAttachmentLink(reminder Reminder, ids []string,
	now time.Time,
) (cloudkit.ReminderAttachmentParentOperation, error) {
	operation := new(cloudkit.ReminderAttachmentParentOperation)

	tokens, err := sdk.reminderTokens(sdk.clock(), reminderLinkedOperationCount)
	if err != nil {
		return *operation, err
	}

	resolution := new(cloudkit.ReminderAttachmentLinkTokensMap)
	resolution.Map.AttachmentIDs = tokens[0]
	resolution.Map.LastModifiedDate = tokens[1]

	encoded, err := reminderTokenString(resolution)
	if err != nil {
		return *operation, err
	}

	fields := cloudkit.ReminderAttachmentLinkFields{
		AttachmentIDs: cloudkit.ReminderWriteStringList{
			Type: cloudkit.ReminderWriteStringListTypeSTRINGLIST, Value: ids},
		ResolutionTokenMap: encoded, LastModifiedDate: reminderRequiredTimestamp(now)}

	operation.OperationType = cloudkit.ReminderAttachmentParentUpdate
	operation.Record = cloudkit.ReminderAttachmentParentRecord{RecordName: reminderRecordName(reminder.ID),
		RecordType: cloudkit.ReminderAttachmentParentRecordType, Fields: fields,
		PluginFields: map[string]any{}, RecordChangeTag: reminderRevisionPointer(reminder.RecordChangeTag)}

	return *operation, nil
}
