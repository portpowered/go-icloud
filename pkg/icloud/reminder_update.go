package icloud

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const (
	updateReminderOperation  = "UpdateReminder"
	reminderUpdateTokenCount = 11
)

// UpdateReminder writes a complete reminder snapshot without mutating caller-owned state.
func (sdk *SDK) UpdateReminder(ctx context.Context, request UpdateReminderRequest) (*ReminderMutationResult, error) {
	err := ctx.Err()
	if err != nil {
		return nil, driveContextFailure(updateReminderOperation, err)
	}

	auth, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(updateReminderOperation, Configuration, 0, nil, nil, err)
	}

	auth.Origin = request.Auth.RemindersServiceURL
	reminder := copyReminder(request.Reminder)
	now := sdk.clock()

	input, err := sdk.reminderUpdateRequest(reminder, now)
	if err != nil {
		return nil, newClientError(updateReminderOperation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.UpdateReminder(ctx, auth, input)
	if err != nil {
		return nil, adaptFailure(updateReminderOperation, err)
	}

	ack, err := reminderDeletionResult(DeleteReminderRequest{Auth: request.Auth,
		ReminderID: reminder.ID, RecordChangeTag: reminder.RecordChangeTag}, reminderRecordName(reminder.ID), now, response)
	if err != nil {
		return nil, renameReminderFailure(updateReminderOperation, err)
	}

	reminder.RecordChangeTag = ack.RecordChangeTag
	reminder.Modified.Set(ack.Modified)
	applyReminderCompletion(&reminder, now)

	return &ReminderMutationResult{Reminder: reminder, Responses: ack.Responses}, nil
}

func copyReminder(input Reminder) Reminder {
	input.AlarmIDs = reminderCopiedIDs(input.AlarmIDs)
	input.AttachmentIDs = reminderCopiedIDs(input.AttachmentIDs)
	input.HashtagIDs = reminderCopiedIDs(input.HashtagIDs)
	input.RecurrenceRuleIDs = reminderCopiedIDs(input.RecurrenceRuleIDs)
	input.CompletedDate = reminderCopiedNullable(input.CompletedDate)
	input.Created = reminderCopiedNullable(input.Created)
	input.DueDate = reminderCopiedNullable(input.DueDate)
	input.Modified = reminderCopiedNullable(input.Modified)
	input.ParentReminderID = reminderCopiedNullable(input.ParentReminderID)
	input.RecordChangeTag = reminderCopiedNullable(input.RecordChangeTag)
	input.StartDate = reminderCopiedNullable(input.StartDate)
	input.TimeZone = reminderCopiedNullable(input.TimeZone)

	return input
}

func reminderRecordName(name string) string {
	if name != "" && !strings.HasPrefix(name, protocol.RemindersReminderIDPrefixValue) {
		return protocol.RemindersReminderIDPrefixValue + name
	}

	return name
}

func applyReminderCompletion(reminder *Reminder, now time.Time) {
	switch {
	case !reminder.Completed:
		reminder.CompletedDate.SetNull()
	case !reminder.CompletedDate.IsSpecified() || reminder.CompletedDate.IsNull():
		reminder.CompletedDate.Set(reminderMutationInstant(now))
	}
}

func renameReminderFailure(operation string, err error) error {
	var failure *ClientError
	if !errors.As(err, &failure) {
		return err
	}

	failure.operation = operation

	return failure
}

func (sdk *SDK) reminderUpdateRequest(reminder Reminder, now time.Time) (cloudkit.ReminderUpdateRequest, error) {
	fields, err := sdk.reminderUpdateFields(reminder, now)
	if err != nil {
		return cloudkit.ReminderUpdateRequest{}, err
	}

	record := cloudkit.ReminderUpdateRecord{RecordName: reminderRecordName(reminder.ID),
		RecordType: cloudkit.ReminderUpdateRecordTypeReminder, Fields: fields, PluginFields: map[string]any{},
		RecordChangeTag: nil, Parent: nil}

	if reminder.RecordChangeTag.IsSpecified() && !reminder.RecordChangeTag.IsNull() {
		tag := reminder.RecordChangeTag.GetOrEmpty()
		record.RecordChangeTag = &tag
	}

	return cloudkit.ReminderUpdateRequest{Operations: []cloudkit.ReminderUpdateOperation{{
		OperationType: cloudkit.ReminderUpdateOperationType, Record: record}}, ZoneID: reminderWriteZone()}, nil
}

func (sdk *SDK) reminderUpdateFields(reminder Reminder, now time.Time) (cloudkit.ReminderUpdateFields, error) {
	title, notes, err := reminderDocuments(reminder.Title, reminder.Description)
	if err != nil {
		return cloudkit.ReminderUpdateFields{}, err
	}

	tokens, err := sdk.reminderTokens(sdk.clock(), reminderUpdateTokenCount)
	if err != nil {
		return cloudkit.ReminderUpdateFields{}, err
	}

	resolution, err := reminderTokenString(cloudkit.ReminderUpdateTokensMap{Map: cloudkit.ReminderUpdateTokens{
		TitleDocument: tokens[0], NotesDocument: tokens[1], Completed: tokens[2], CompletionDate: tokens[3],
		Priority: tokens[4], Flagged: tokens[5], AllDay: tokens[6], LastModifiedDate: tokens[7],
		DueDate: tokens[8], TimeZone: tokens[9], ParentReminder: tokens[10]}})
	if err != nil {
		return cloudkit.ReminderUpdateFields{}, err
	}

	applyReminderCompletion(&reminder, now)

	fields := cloudkit.ReminderUpdateFields{TitleDocument: reminderDocument(title), NotesDocument: reminderDocument(notes),
		Completed: reminderBoolean(reminder.Completed), CompletionDate: cloudkit.ReminderOptionalTimestamp{
			Type: cloudkit.ReminderOptionalTIMESTAMPType, Value: nil}, Priority: reminderWriteInteger(reminder.Priority),
		Flagged: reminderBoolean(reminder.Flagged), AllDay: reminderBoolean(reminder.AllDay),
		LastModifiedDate: reminderTimestamp(now), DueDate: cloudkit.ReminderOptionalTimestamp{
			Type: cloudkit.ReminderOptionalTIMESTAMPType, Value: nil}, TimeZone: cloudkit.ReminderOptionalString{
			Type: cloudkit.ReminderOptionalSTRINGType, Value: nil},
		ParentReminder:     reminderWriteReference(reminderRecordName(reminder.ParentReminderID.GetOrEmpty())),
		ResolutionTokenMap: resolution}
	if reminder.CompletedDate.IsSpecified() && !reminder.CompletedDate.IsNull() {
		fields.CompletionDate = reminderTimestamp(reminder.CompletedDate.GetOrEmpty())
	}

	if reminder.DueDate.IsSpecified() && !reminder.DueDate.IsNull() {
		fields.DueDate = reminderTimestamp(reminder.DueDate.GetOrEmpty())
	}

	if reminder.TimeZone.GetOrEmpty() != "" {
		fields.TimeZone = reminderString(reminder.TimeZone.GetOrEmpty())
	}

	return fields, nil
}
