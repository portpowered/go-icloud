package icloud

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const deleteReminderAttachmentOperation = "DeleteReminderAttachment"

var errAttachmentParent = errors.New("attachment belongs to another reminder")

// DeleteReminderAttachment soft-deletes the record and atomically removes every matching parent link.
func (sdk *SDK) DeleteReminderAttachment(ctx context.Context,
	request DeleteReminderAttachmentRequest,
) (*ReminderAttachmentMutationResult, error) {
	auth, err := reminderWriteContext(ctx, request.Auth, deleteReminderAttachmentOperation)
	if err != nil {
		return nil, err
	}

	name, parentID, tag, err := attachmentIdentity(request.Attachment)
	if err != nil {
		return nil, newClientError(deleteReminderAttachmentOperation, Configuration, 0, nil, nil, err)
	}

	reminder := copyReminder(request.Reminder)
	if parentID != "" && reminderRecordName(parentID) != reminderRecordName(reminder.ID) {
		return nil, newClientError(deleteReminderAttachmentOperation, Configuration, 0, nil, nil, errAttachmentParent)
	}

	raw := strings.TrimPrefix(name, protocol.RemindersAttachmentIDPrefixValue)
	reminder.AttachmentIDs = reminderUnlinkedAttachmentIDs(reminder.AttachmentIDs, raw)

	input, err := sdk.reminderAttachmentDeletionRequest(reminder, name, parentID, tag)
	if err != nil {
		return nil, newClientError(deleteReminderAttachmentOperation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.ModifyReminderAttachment(ctx, auth, input)
	if err != nil {
		return nil, adaptFailure(deleteReminderAttachmentOperation, err)
	}

	tags, err := reminderAcknowledgements(deleteReminderAttachmentOperation, response)
	if err != nil {
		return nil, err
	}

	reminder.RecordChangeTag = reminderAcknowledgedTag(reminder.RecordChangeTag, reminderRecordName(reminder.ID), tags)
	projected := request.Attachment

	err = attachmentSetTag(&projected, reminderAcknowledgedTag(tag, name, tags))
	if err != nil {
		return nil, newClientError(deleteReminderAttachmentOperation, InvalidResponse, 0, nil, nil, err)
	}

	return &ReminderAttachmentMutationResult{Reminder: reminder, Attachment: projected,
		Responses: []ResponseMetadata{publicMetadata(response.Metadata)}}, nil
}

func reminderUnlinkedAttachmentIDs(input []string, removed string) []string {
	result := make([]string, 0, len(input))

	for _, name := range input {
		raw := strings.TrimPrefix(name, protocol.RemindersAttachmentIDPrefixValue)
		if raw != removed {
			result = append(result, raw)
		}
	}

	return result
}

func attachmentIdentity(input ReminderAttachment) (string, string, nullable.Nullable[string], error) {
	if attachmentSelection(input) == nil {
		return "", "", nil, errAttachmentFields
	}

	if attachmentIsURL(input) {
		url, err := input.AsReminderURLAttachment()

		return reminderRelatedRecordName(url.ID, protocol.RemindersAttachmentIDPrefixValue),
			url.ReminderID, url.RecordChangeTag, err
	}

	image, err := input.AsReminderImageAttachment()

	return reminderRelatedRecordName(image.ID, protocol.RemindersAttachmentIDPrefixValue),
		image.ReminderID, image.RecordChangeTag, err
}

func (sdk *SDK) reminderAttachmentDeletionRequest(reminder Reminder, name, parentID string,
	tag nullable.Nullable[string],
) (cloudkit.RemindersModificationRequest, error) {
	input := new(cloudkit.RemindersModificationRequest)

	parent, err := sdk.reminderAttachmentLink(reminder, reminder.AttachmentIDs, sdk.clock())
	if err != nil {
		return *input, err
	}

	fields := cloudkit.ReminderAttachmentDeletionFields{
		Deleted: cloudkit.ReminderAttachmentDeletedValue{Type: cloudkit.ReminderAttachmentDeletedInteger,
			Value: cloudkit.ReminderAttachmentDeleted}, Reminder: attachmentParentReference(parentID)}
	record := cloudkit.ReminderAttachmentDeletionRecord{RecordName: name,
		RecordType: cloudkit.ReminderAttachmentDeletionRecordTypeAttachment, Fields: fields,
		PluginFields: map[string]any{}, RecordChangeTag: reminderRevisionPointer(tag)}
	child := cloudkit.ReminderAttachmentDeletionOperation{
		OperationType: cloudkit.ReminderAttachmentDeletionOperationType, Record: record}
	operations := make([]cloudkit.ReminderAttachmentDeletionRequest_Operations_Item, reminderLinkedOperationCount)

	err = operations[0].FromReminderAttachmentParentOperation(parent)
	if err == nil {
		err = operations[1].FromReminderAttachmentDeletionOperation(child)
	}

	if err == nil {
		err = input.FromReminderAttachmentDeletionRequest(cloudkit.ReminderAttachmentDeletionRequest{
			Operations: operations, ZoneID: reminderWriteZone(), Atomic: cloudkit.ReminderAttachmentDeletionAtomic})
	}

	if err != nil {
		return *input, fmt.Errorf("encode attachment deletion: %w", err)
	}

	return *input, nil
}
