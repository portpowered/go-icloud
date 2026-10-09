package icloud

import (
	"context"
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const createReminderAttachmentOperation = "CreateReminderURLAttachment"

// CreateReminderURLAttachment creates a URL record and atomically links it to its reminder.
func (sdk *SDK) CreateReminderURLAttachment(ctx context.Context,
	request CreateReminderURLAttachmentRequest,
) (*ReminderAttachmentMutationResult, error) {
	auth, err := reminderWriteContext(ctx, request.Auth, createReminderAttachmentOperation)
	if err != nil {
		return nil, err
	}

	identity, err := sdk.randomUUID()
	if err != nil {
		return nil, newClientError(createReminderAttachmentOperation, Configuration, 0, nil, nil, err)
	}

	reminder := copyReminder(request.Reminder)
	reminder.AttachmentIDs = reminderRelatedIDs(reminder.AttachmentIDs, protocol.RemindersAttachmentIDPrefixValue, "")
	reminder.AttachmentIDs = append(reminder.AttachmentIDs, identity)
	attachment := new(ReminderURLAttachment)
	attachment.ID = protocol.RemindersAttachmentIDPrefixValue + identity
	attachment.ReminderID = reminderRecordName(reminder.ID)
	attachment.URL = request.URL

	attachment.UTI = protocol.RemindersURLAttachmentUTIValue

	if request.UTI != nil {
		attachment.UTI = *request.UTI
	}

	input, err := sdk.reminderAttachmentCreationRequest(reminder, *attachment)
	if err != nil {
		return nil, newClientError(createReminderAttachmentOperation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.ModifyReminderAttachment(ctx, auth, input)
	if err != nil {
		return nil, adaptFailure(createReminderAttachmentOperation, err)
	}

	tags, err := reminderAcknowledgements(createReminderAttachmentOperation, response)
	if err != nil {
		return nil, err
	}

	reminder.RecordChangeTag = reminderAcknowledgedTag(reminder.RecordChangeTag, reminderRecordName(reminder.ID), tags)
	attachment.RecordChangeTag = reminderAcknowledgedTag(nil, attachment.ID, tags)
	projected := new(ReminderAttachment)

	err = projected.FromReminderURLAttachment(*attachment)
	if err != nil {
		return nil, newClientError(createReminderAttachmentOperation, InvalidResponse, 0, nil, nil, err)
	}

	return &ReminderAttachmentMutationResult{Reminder: reminder, Attachment: *projected,
		Responses: []ResponseMetadata{publicMetadata(response.Metadata)}}, nil
}

func (sdk *SDK) reminderAttachmentCreationRequest(reminder Reminder,
	attachment ReminderURLAttachment,
) (cloudkit.RemindersModificationRequest, error) {
	input := new(cloudkit.RemindersModificationRequest)

	parent, err := sdk.reminderAttachmentLink(reminder, reminder.AttachmentIDs, sdk.clock())
	if err != nil {
		return *input, err
	}

	fields := cloudkit.ReminderAttachmentURLCreationFields{Type: reminderAttachmentURLType(),
		Reminder: attachmentRequiredParent(attachment.ReminderID), URL: attachmentEncryptedURL(attachment.URL),
		UTI: attachmentWriteString(attachment.UTI), Imported: attachmentZero(), Deleted: attachmentZero()}
	record := cloudkit.ReminderAttachmentURLCreationRecord{RecordName: attachment.ID,
		RecordType: cloudkit.ReminderAttachmentURLCreationRecordTypeAttachment, Fields: fields,
		PluginFields: map[string]any{}, RecordChangeTag: nil,
		Parent: cloudkit.CKWriteParent{RecordName: attachment.ReminderID, AdditionalProperties: nil}}
	child := cloudkit.ReminderAttachmentURLCreationOperation{
		OperationType: cloudkit.ReminderAttachmentURLCreationOperationType, Record: record}
	operations := make([]cloudkit.ReminderAttachmentURLCreationRequest_Operations_Item, reminderLinkedOperationCount)

	err = operations[0].FromReminderAttachmentParentOperation(parent)
	if err == nil {
		err = operations[1].FromReminderAttachmentURLCreationOperation(child)
	}

	if err == nil {
		err = input.FromReminderAttachmentURLCreationRequest(cloudkit.ReminderAttachmentURLCreationRequest{
			Operations: operations, ZoneID: reminderWriteZone(), Atomic: cloudkit.ReminderAttachmentURLCreationAtomic})
	}

	if err != nil {
		return *input, fmt.Errorf("encode attachment creation: %w", err)
	}

	return *input, nil
}
