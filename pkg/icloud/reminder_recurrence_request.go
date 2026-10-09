package icloud

import (
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func (sdk *SDK) reminderRecurrenceParent(reminder Reminder) (cloudkit.ReminderRecurrenceParentOperation, error) {
	now := sdk.clock()

	tokens, err := sdk.reminderTokens(sdk.clock(), reminderLinkedOperationCount)
	if err != nil {
		return cloudkit.ReminderRecurrenceParentOperation{}, err
	}

	tokenMap := cloudkit.ReminderRecurrenceLinkTokensMap{Map: cloudkit.ReminderRecurrenceLinkTokens{
		RecurrenceRuleIDs: tokens[0], LastModifiedDate: tokens[1]}}

	encoded, err := reminderTokenString(tokenMap)
	if err != nil {
		return cloudkit.ReminderRecurrenceParentOperation{}, err
	}

	fields := cloudkit.ReminderRecurrenceLinkFields{RecurrenceRuleIDs: cloudkit.ReminderWriteStringList{
		Type: cloudkit.ReminderWriteStringListTypeSTRINGLIST, Value: reminder.RecurrenceRuleIDs},
		ResolutionTokenMap: encoded, LastModifiedDate: reminderRequiredTimestamp(now)}
	record := cloudkit.ReminderRecurrenceParentRecord{RecordName: reminderRecordName(reminder.ID),
		RecordType: cloudkit.ReminderRecurrenceParentRecordTypeReminder, Fields: fields,
		PluginFields: map[string]any{}, RecordChangeTag: reminderRevisionPointer(reminder.RecordChangeTag)}

	return cloudkit.ReminderRecurrenceParentOperation{
		OperationType: cloudkit.ReminderRecurrenceParentOperationTypeUpdate, Record: record}, nil
}

func (sdk *SDK) reminderRecurrenceCreationRequest(reminder Reminder,
	rule ReminderRecurrenceRule,
) (cloudkit.ReminderRecurrenceCreationRequest, error) {
	parent, err := sdk.reminderRecurrenceParent(reminder)
	if err != nil {
		return cloudkit.ReminderRecurrenceCreationRequest{}, err
	}

	record := cloudkit.ReminderRecurrenceCreationRecord{RecordName: rule.ID,
		RecordType: cloudkit.ReminderRecurrenceCreationRecordTypeRecurrenceRule,
		Fields:     reminderRecurrenceCreationFields(rule), PluginFields: map[string]any{}, RecordChangeTag: nil,
		Parent: cloudkit.CKWriteParent{RecordName: rule.ReminderID, AdditionalProperties: nil}}
	child := cloudkit.ReminderRecurrenceCreationOperation{
		OperationType: cloudkit.ReminderRecurrenceCreationOperationTypeCreate, Record: record}
	operations := make([]cloudkit.ReminderRecurrenceCreationRequest_Operations_Item, reminderLinkedOperationCount)

	err = operations[0].FromReminderRecurrenceParentOperation(parent)
	if err != nil {
		return cloudkit.ReminderRecurrenceCreationRequest{}, fmt.Errorf("encode recurrence link: %w", err)
	}

	err = operations[1].FromReminderRecurrenceCreationOperation(child)
	if err != nil {
		return cloudkit.ReminderRecurrenceCreationRequest{}, fmt.Errorf("encode recurrence creation: %w", err)
	}

	return cloudkit.ReminderRecurrenceCreationRequest{Operations: operations, ZoneID: reminderWriteZone(),
		Atomic: cloudkit.ReminderRecurrenceCreationRequestAtomicTrue}, nil
}

func reminderRecurrenceCreationFields(rule ReminderRecurrenceRule) cloudkit.ReminderRecurrenceCreationFields {
	return cloudkit.ReminderRecurrenceCreationFields{Reminder: reminderRecurrenceReference(rule.ReminderID),
		Frequency: cloudkit.ReminderRecurrenceWriteFrequency{Type: cloudkit.ReminderRecurrenceWriteFrequencyTypeINT64,
			Value: cloudkit.ReminderRecurrenceWriteFrequencyValue(rule.Frequency)},
		Interval: cloudkit.ReminderRecurrenceWriteInterval{Type: cloudkit.ReminderRecurrenceWriteIntervalTypeINT64,
			Value: rule.Interval}, OccurrenceCount: cloudkit.ReminderRecurrenceWriteCount{
			Type: cloudkit.ReminderRecurrenceWriteCountTypeINT64, Value: rule.OccurrenceCount},
		FirstDayOfTheWeek: cloudkit.ReminderRecurrenceWriteWeekday{Type: cloudkit.ReminderRecurrenceWriteWeekdayTypeINT64,
			Value: rule.FirstDayOfWeek}, Imported: cloudkit.ReminderRelationZero{Type: cloudkit.ReminderRelationZeroTypeINT64,
			Value: cloudkit.ReminderRelationZero0}, Deleted: cloudkit.ReminderRelationZero{
			Type:  cloudkit.ReminderRelationZeroTypeINT64,
			Value: cloudkit.ReminderRelationZero0}}
}

func reminderRecurrenceUpdateRequest(
	request UpdateReminderRecurrenceRuleRequest,
) cloudkit.ReminderRecurrenceUpdateRequest {
	record := cloudkit.ReminderRecurrenceUpdateRecord{RecordName: reminderRelatedRecordName(request.RecurrenceRule.ID,
		protocol.RemindersRecurrenceRuleIDPrefixValue), RecordType: cloudkit.ReminderRecurrenceUpdateRecordTypeRecurrenceRule,
		Fields: reminderRecurrenceUpdateFields(request), PluginFields: map[string]any{},
		RecordChangeTag: reminderRevisionPointer(request.RecurrenceRule.RecordChangeTag)}

	return cloudkit.ReminderRecurrenceUpdateRequest{Operations: []cloudkit.ReminderRecurrenceUpdateOperation{{
		OperationType: cloudkit.ReminderRecurrenceUpdateOperationTypeUpdate, Record: record}}, ZoneID: reminderWriteZone()}
}

func reminderRecurrenceUpdateFields(
	request UpdateReminderRecurrenceRuleRequest,
) cloudkit.ReminderRecurrenceUpdateFields {
	fields := cloudkit.ReminderRecurrenceUpdateFields{Frequency: nil, Interval: nil, OccurrenceCount: nil,
		FirstDayOfTheWeek: nil, Reminder: nil}
	if request.Frequency != nil {
		fields.Frequency = &cloudkit.ReminderRecurrenceWriteFrequency{
			Type:  cloudkit.ReminderRecurrenceWriteFrequencyTypeINT64,
			Value: cloudkit.ReminderRecurrenceWriteFrequencyValue(*request.Frequency)}
	}

	if request.Interval != nil {
		fields.Interval = &cloudkit.ReminderRecurrenceWriteInterval{
			Type: cloudkit.ReminderRecurrenceWriteIntervalTypeINT64, Value: *request.Interval}
	}

	if request.OccurrenceCount != nil {
		fields.OccurrenceCount = &cloudkit.ReminderRecurrenceWriteCount{
			Type: cloudkit.ReminderRecurrenceWriteCountTypeINT64, Value: *request.OccurrenceCount}
	}

	if request.FirstDayOfWeek != nil {
		fields.FirstDayOfTheWeek = &cloudkit.ReminderRecurrenceWriteWeekday{
			Type: cloudkit.ReminderRecurrenceWriteWeekdayTypeINT64, Value: *request.FirstDayOfWeek}
	}

	if request.RecurrenceRule.ReminderID != "" {
		reference := reminderRecurrenceReference(reminderRecordName(request.RecurrenceRule.ReminderID))
		fields.Reminder = &reference
	}

	return fields
}

func (sdk *SDK) reminderRecurrenceDeletionRequest(reminder Reminder,
	rule ReminderRecurrenceRule,
) (cloudkit.ReminderRecurrenceDeletionRequest, error) {
	parent, err := sdk.reminderRecurrenceParent(reminder)
	if err != nil {
		return cloudkit.ReminderRecurrenceDeletionRequest{}, err
	}

	fields := cloudkit.ReminderRecurrenceDeletionFields{Deleted: cloudkit.ReminderRelationDeleted{
		Type: cloudkit.ReminderRelationDeletedTypeINT64, Value: cloudkit.ReminderRelationDeleted1}, Reminder: nil}

	if rule.ReminderID != "" {
		reference := reminderRecurrenceReference(reminderRecordName(rule.ReminderID))
		fields.Reminder = &reference
	}

	record := cloudkit.ReminderRecurrenceDeletionRecord{RecordName: reminderRelatedRecordName(rule.ID,
		protocol.RemindersRecurrenceRuleIDPrefixValue),
		RecordType: cloudkit.ReminderRecurrenceDeletionRecordTypeRecurrenceRule,
		Fields:     fields, PluginFields: map[string]any{}, RecordChangeTag: reminderRevisionPointer(rule.RecordChangeTag)}
	operations := make([]cloudkit.ReminderRecurrenceDeletionRequest_Operations_Item, reminderLinkedOperationCount)

	err = operations[0].FromReminderRecurrenceParentOperation(parent)
	if err != nil {
		return cloudkit.ReminderRecurrenceDeletionRequest{}, fmt.Errorf("encode recurrence unlink: %w", err)
	}

	err = operations[1].FromReminderRecurrenceDeletionOperation(cloudkit.ReminderRecurrenceDeletionOperation{
		OperationType: cloudkit.ReminderRecurrenceDeletionOperationTypeUpdate, Record: record})
	if err != nil {
		return cloudkit.ReminderRecurrenceDeletionRequest{}, fmt.Errorf("encode recurrence deletion: %w", err)
	}

	return cloudkit.ReminderRecurrenceDeletionRequest{Operations: operations, ZoneID: reminderWriteZone(),
		Atomic: cloudkit.ReminderRecurrenceDeletionRequestAtomicTrue}, nil
}

func reminderRecurrenceReference(name string) cloudkit.ReminderRecurrenceReference {
	return cloudkit.ReminderRecurrenceReference{Type: cloudkit.ReminderRecurrenceReferenceTypeREFERENCE,
		Value: cloudkit.ReminderWriteReferenceValue{RecordName: name,
			Action: cloudkit.ReminderWriteReferenceValueActionVALIDATE}}
}
