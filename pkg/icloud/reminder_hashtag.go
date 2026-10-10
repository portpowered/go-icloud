package icloud

import (
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func reminderHashtagName(name string) cloudkit.ReminderHashtagWriteName {
	return cloudkit.ReminderHashtagWriteName{Type: cloudkit.ReminderHashtagWriteNameTypeEncryptedBytes,
		Value: []byte(name)}
}

func copyReminderHashtag(input ReminderHashtag) ReminderHashtag {
	input.Created = reminderCopiedNullable(input.Created)
	input.RecordChangeTag = reminderCopiedNullable(input.RecordChangeTag)

	return input
}

func (sdk *SDK) reminderHashtagParent(reminder Reminder, ids []string,
) (cloudkit.ReminderHashtagParentOperation, error) {
	now := sdk.clock()
	tokenTime := sdk.clock()

	var tokens cloudkit.ReminderHashtagLinkTokensMap

	var err error

	tokens.Map.HashtagIDs, err = sdk.reminderResolutionToken(tokenTime)
	if err != nil {
		return cloudkit.ReminderHashtagParentOperation{}, err
	}

	tokens.Map.LastModifiedDate, err = sdk.reminderResolutionToken(tokenTime)
	if err != nil {
		return cloudkit.ReminderHashtagParentOperation{}, err
	}

	encoded, err := reminderTokenString(tokens)
	if err != nil {
		return cloudkit.ReminderHashtagParentOperation{}, err
	}

	return cloudkit.ReminderHashtagParentOperation{
		OperationType: cloudkit.ReminderHashtagParentOperationType,
		Record: cloudkit.ReminderHashtagParentRecord{RecordName: reminderRecordName(reminder.ID),
			RecordType: cloudkit.ReminderHashtagParentRecordTypeReminder, PluginFields: map[string]any{},
			RecordChangeTag: reminderRevisionPointer(reminder.RecordChangeTag),
			Fields: cloudkit.ReminderHashtagLinkFields{HashtagIDs: cloudkit.ReminderWriteStringList{
				Type: cloudkit.ReminderWriteStringListTypeSTRINGLIST, Value: ids}, ResolutionTokenMap: encoded,
				LastModifiedDate: reminderRequiredTimestamp(now)}}}, nil
}
