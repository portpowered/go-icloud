package icloud

import (
	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func reminderImageAttachmentUpdate(request UpdateReminderAttachmentRequest, attachment ReminderImageAttachment) (
	cloudkit.RemindersModificationRequest, ReminderAttachment, string, nullable.Nullable[string], error,
) {
	input := new(cloudkit.RemindersModificationRequest)
	projected := new(ReminderAttachment)

	fields, err := reminderImageAttachmentFields(request, &attachment)
	if err != nil {
		return *input, *projected, "", nil, err
	}

	name := reminderRelatedRecordName(attachment.ID, string(cloudkit.AttachmentIDPrefixAttachment))
	record := cloudkit.ReminderAttachmentImageUpdateRecord{RecordName: name,
		RecordType: cloudkit.ReminderAttachmentImageUpdateRecordTypeAttachment, Fields: fields,
		PluginFields: map[string]any{}, RecordChangeTag: reminderRevisionPointer(attachment.RecordChangeTag)}
	operation := cloudkit.ReminderAttachmentImageUpdateOperation{
		OperationType: cloudkit.ReminderAttachmentImageUpdateOperationType, Record: record}

	err = input.FromReminderAttachmentImageUpdateRequest(cloudkit.ReminderAttachmentImageUpdateRequest{
		Operations: []cloudkit.ReminderAttachmentImageUpdateOperation{operation}, ZoneID: reminderWriteZone()})
	if err == nil {
		err = projected.FromReminderImageAttachment(attachment)
	}

	return *input, *projected, name, attachment.RecordChangeTag, err
}

func reminderImageAttachmentFields(request UpdateReminderAttachmentRequest,
	attachment *ReminderImageAttachment,
) (cloudkit.ReminderAttachmentImageUpdateFields, error) {
	fields := new(cloudkit.ReminderAttachmentImageUpdateFields)
	if request.UTI == nil && request.Filename == nil && request.FileSize == nil &&
		request.Width == nil && request.Height == nil {
		return *fields, errAttachmentFields
	}

	fields.UTI = attachmentStringUpdate(request.UTI, &attachment.UTI)
	fields.FileName = attachmentStringUpdate(request.Filename, &attachment.Filename)
	fields.FileSize = attachmentIntegerUpdate(request.FileSize, &attachment.FileSize)
	fields.Width = attachmentIntegerUpdate(request.Width, &attachment.Width)

	fields.Height = attachmentIntegerUpdate(request.Height, &attachment.Height)

	if attachment.FileSize < 0 || attachment.Width < 0 || attachment.Height < 0 {
		return *fields, errAttachmentFields
	}

	fields.Type = cloudkit.ReminderAttachmentImageType{Type: cloudkit.ReminderAttachmentImageTypeString,
		Value: cloudkit.ReminderAttachmentImageValue}
	fields.Reminder = attachmentParentReference(attachment.ReminderID)

	return *fields, nil
}

func attachmentStringUpdate(value *string, target *string) *cloudkit.ReminderWriteString {
	if value == nil {
		return nil
	}

	*target = *value
	field := attachmentWriteString(*value)

	return &field
}

func attachmentIntegerUpdate(value *int64, target *int64) *cloudkit.ReminderAttachmentNonnegativeInteger {
	if value == nil {
		return nil
	}

	*target = *value
	field := cloudkit.ReminderAttachmentNonnegativeInteger{
		Type: cloudkit.ReminderAttachmentNonnegativeIntegerTypeValue, Value: *value}

	return &field
}
