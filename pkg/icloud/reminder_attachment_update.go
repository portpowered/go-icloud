package icloud

import (
	"context"
	"errors"

	"github.com/oapi-codegen/nullable"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const updateReminderAttachmentOperation = "UpdateReminderAttachment"

var errAttachmentFields = errors.New("no attachment fields provided or invalid image dimensions")

// UpdateReminderAttachment updates URL text or image metadata without mutating the caller's snapshot.
func (sdk *SDK) UpdateReminderAttachment(ctx context.Context,
	request UpdateReminderAttachmentRequest,
) (*UpdateReminderAttachmentResult, error) {
	auth, err := reminderWriteContext(ctx, request.Auth, updateReminderAttachmentOperation)
	if err != nil {
		return nil, err
	}

	input, projected, name, tag, err := reminderAttachmentUpdate(request)
	if err != nil {
		return nil, newClientError(updateReminderAttachmentOperation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.ModifyReminderAttachment(ctx, auth, input)
	if err != nil {
		return nil, adaptFailure(updateReminderAttachmentOperation, err)
	}

	tags, err := reminderAcknowledgements(updateReminderAttachmentOperation, response)
	if err != nil {
		return nil, err
	}

	err = attachmentSetTag(&projected, reminderAcknowledgedTag(tag, name, tags))
	if err != nil {
		return nil, newClientError(updateReminderAttachmentOperation, InvalidResponse, 0, nil, nil, err)
	}

	return &UpdateReminderAttachmentResult{Attachment: projected,
		Responses: []ResponseMetadata{publicMetadata(response.Metadata)}}, nil
}

func reminderAttachmentUpdate(request UpdateReminderAttachmentRequest) (
	cloudkit.RemindersModificationRequest, ReminderAttachment, string, nullable.Nullable[string], error,
) {
	if attachmentSelection(request.Attachment) == nil {
		return cloudkit.RemindersModificationRequest{}, ReminderAttachment{}, "", nil, errAttachmentFields
	}

	url, err := request.Attachment.AsReminderURLAttachment()
	if err == nil && attachmentIsURL(request.Attachment) {
		return reminderURLAttachmentUpdate(request, url)
	}

	image, err := request.Attachment.AsReminderImageAttachment()
	if err != nil {
		return cloudkit.RemindersModificationRequest{}, ReminderAttachment{}, "", nil, err
	}

	return reminderImageAttachmentUpdate(request, image)
}

func reminderURLAttachmentUpdate(request UpdateReminderAttachmentRequest, attachment ReminderURLAttachment) (
	cloudkit.RemindersModificationRequest, ReminderAttachment, string, nullable.Nullable[string], error,
) {
	input := new(cloudkit.RemindersModificationRequest)

	projected := new(ReminderAttachment)

	if request.URL == nil && request.UTI == nil {
		return *input, *projected, "", nil, errAttachmentFields
	}

	fields := new(cloudkit.ReminderAttachmentURLUpdateFields)
	fields.Type = reminderAttachmentURLType()

	fields.Reminder = attachmentParentReference(attachment.ReminderID)

	if request.URL != nil {
		attachment.URL = *request.URL
		value := attachmentEncryptedURL(*request.URL)
		fields.URL = &value
	}

	if request.UTI != nil {
		attachment.UTI = *request.UTI
		value := attachmentWriteString(*request.UTI)
		fields.UTI = &value
	}

	name := reminderRelatedRecordName(attachment.ID, string(cloudkit.AttachmentIDPrefixAttachment))
	record := cloudkit.ReminderAttachmentURLUpdateRecord{RecordName: name,
		RecordType: cloudkit.ReminderAttachmentURLUpdateRecordTypeAttachment, Fields: *fields,
		PluginFields: map[string]any{}, RecordChangeTag: reminderRevisionPointer(attachment.RecordChangeTag)}
	operation := cloudkit.ReminderAttachmentURLUpdateOperation{
		OperationType: cloudkit.ReminderAttachmentURLUpdateOperationType, Record: record}

	err := input.FromReminderAttachmentURLUpdateRequest(cloudkit.ReminderAttachmentURLUpdateRequest{
		Operations: []cloudkit.ReminderAttachmentURLUpdateOperation{operation}, ZoneID: reminderWriteZone()})
	if err == nil {
		err = projected.FromReminderURLAttachment(attachment)
	}

	return *input, *projected, name, attachment.RecordChangeTag, err
}
