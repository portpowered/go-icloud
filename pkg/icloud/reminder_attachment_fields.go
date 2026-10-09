package icloud

import (
	"encoding/json"

	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func attachmentIsURL(input ReminderAttachment) bool {
	projection := attachmentSelection(input)

	return projection != nil && projection.URL != nil && projection.FileAssetURL == nil
}

func attachmentSelection(input ReminderAttachment) *ReminderAttachmentSelection {
	body, err := json.Marshal(input)
	if err != nil {
		return nil
	}

	projection := new(ReminderAttachmentSelection)
	err = json.Unmarshal(body, projection)

	if err != nil || (projection.URL == nil) == (projection.FileAssetURL == nil) {
		return nil
	}

	return projection
}

func attachmentSetTag(input *ReminderAttachment, tag nullable.Nullable[string]) error {
	if attachmentIsURL(*input) {
		url, err := input.AsReminderURLAttachment()
		if err != nil {
			return err
		}

		url.RecordChangeTag = tag

		return input.FromReminderURLAttachment(url)
	}

	image, err := input.AsReminderImageAttachment()
	if err != nil {
		return err
	}

	image.RecordChangeTag = tag

	return input.FromReminderImageAttachment(image)
}

func attachmentParentReference(name string) *cloudkit.ReminderAttachmentReference {
	if name == "" {
		return nil
	}

	value := attachmentRequiredParent(reminderRelatedRecordName(name, protocol.RemindersReminderIDPrefixValue))

	return &value
}

func attachmentRequiredParent(name string) cloudkit.ReminderAttachmentReference {
	return cloudkit.ReminderAttachmentReference{Type: cloudkit.ReminderAttachmentReferenceTypeValue,
		Value: cloudkit.ReminderWriteReferenceValue{RecordName: name,
			Action: cloudkit.ReminderWriteReferenceValueActionVALIDATE}}
}

func attachmentZero() cloudkit.ReminderAttachmentZeroValue {
	return cloudkit.ReminderAttachmentZeroValue{Type: cloudkit.ReminderAttachmentZeroInteger,
		Value: cloudkit.ReminderAttachmentZero}
}

func attachmentWriteString(value string) cloudkit.ReminderWriteString {
	return cloudkit.ReminderWriteString{Type: cloudkit.ReminderWriteStringTypeSTRING, Value: value}
}

func attachmentEncryptedURL(value string) cloudkit.ReminderAttachmentEncryptedURL {
	return cloudkit.ReminderAttachmentEncryptedURL{Type: cloudkit.ReminderAttachmentURLString,
		Value: value, IsEncrypted: cloudkit.ReminderAttachmentURLEncrypted}
}

func reminderAttachmentURLType() cloudkit.ReminderAttachmentURLType {
	return cloudkit.ReminderAttachmentURLType{Type: cloudkit.ReminderAttachmentURLTypeString,
		Value: cloudkit.ReminderAttachmentURLValue}
}
