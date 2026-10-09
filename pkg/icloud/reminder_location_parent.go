package icloud

import (
	"time"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func (sdk *SDK) locationParentOperation(reminder Reminder, now time.Time) (cloudkit.LocationParentOperation, error) {
	const tokenCount = 2

	tokens, err := sdk.reminderTokens(sdk.clock(), tokenCount)
	if err != nil {
		return cloudkit.LocationParentOperation{}, err
	}

	tokenMap := new(cloudkit.ReminderAlarmLinkTokensMap)
	tokenMap.Map.AlarmIDs, tokenMap.Map.LastModifiedDate = tokens[0], tokens[1]

	resolution, err := reminderTokenString(tokenMap)
	if err != nil {
		return cloudkit.LocationParentOperation{}, err
	}

	fields := cloudkit.ReminderAlarmLinkFields{
		AlarmIDs: cloudkit.ReminderWriteStringList{Type: cloudkit.ReminderWriteStringListTypeSTRINGLIST,
			Value: reminder.AlarmIDs}, ResolutionTokenMap: resolution, LastModifiedDate: reminderRequiredTimestamp(now)}

	return cloudkit.LocationParentOperation{OperationType: cloudkit.LocationParentOperationType,
		Record: cloudkit.LocationParentRecord{RecordName: reminderRecordName(reminder.ID),
			RecordType: cloudkit.LocationParentRecordType, Fields: fields,
			PluginFields: map[string]any{}, RecordChangeTag: reminderRevisionPointer(reminder.RecordChangeTag)}}, nil
}
