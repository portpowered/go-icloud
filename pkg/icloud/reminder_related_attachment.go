package icloud

import (
	"encoding/json"
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

//nolint:nilnil // Source intentionally skips unsupported attachment types without failing the query.
func projectReminderAttachment(record cloudkit.CKRecord) (*ReminderAttachment, error) {
	kind, err := reminderRelatedKind(record)
	if err != nil {
		return nil, err
	}

	result := new(ReminderAttachment)

	switch kind {
	case protocol.RemindersURLAttachmentTypeValue:
		value, valueErr := projectReminderURLAttachment(record)
		if valueErr != nil {
			return nil, valueErr
		}

		err = result.FromReminderURLAttachment(value)
	case protocol.RemindersImageAttachmentTypeValue:
		value, valueErr := projectReminderImageAttachment(record)
		if valueErr != nil {
			return nil, valueErr
		}

		err = result.FromReminderImageAttachment(value)
	default:
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("encode reminder attachment: %w", err)
	}

	return result, nil
}

func projectReminderURLAttachment(record cloudkit.CKRecord) (ReminderURLAttachment, error) {
	result := new(ReminderURLAttachment)
	result.ID = record.RecordName
	result.RecordChangeTag = reminderRelatedTag(record)
	result.UTI = protocol.RemindersURLAttachmentUTIValue

	var err error

	result.ReminderID, err = reminderReference(record, protocol.RemindersRelatedFieldReminderValue)
	if err != nil {
		return *result, err
	}

	err = reminderRelatedStrings(record, map[string]*string{
		protocol.RemindersRelatedFieldURLValue: &result.URL,
		protocol.RemindersRelatedFieldUTIValue: &result.UTI})
	if err != nil {
		return *result, err
	}

	bytes, err := reminderRelatedBytes(record, protocol.RemindersRelatedFieldURLValue)
	if !bytes {
		result.URL = reminderAttachmentURL(result.URL)
	}

	return *result, err
}

func projectReminderImageAttachment(record cloudkit.CKRecord) (ReminderImageAttachment, error) {
	result := new(ReminderImageAttachment)
	result.ID = record.RecordName
	result.RecordChangeTag = reminderRelatedTag(record)
	result.UTI = protocol.RemindersImageAttachmentUTIValue

	var err error

	result.ReminderID, err = reminderReference(record, protocol.RemindersRelatedFieldReminderValue)
	if err != nil {
		return *result, err
	}

	err = reminderRelatedStrings(record, map[string]*string{
		protocol.RemindersRelatedFieldFileNameValue: &result.Filename,
		protocol.RemindersRelatedFieldUTIValue:      &result.UTI})
	if err != nil {
		return *result, err
	}

	for name, destination := range map[string]*int64{
		protocol.RemindersRelatedFieldFileSizeValue: &result.FileSize,
		protocol.RemindersRelatedFieldWidthValue:    &result.Width,
		protocol.RemindersRelatedFieldHeightValue:   &result.Height} {
		*destination, err = reminderRelatedInteger(record, name, 0)
		if err != nil || *destination < 0 {
			return *result, errReminderList
		}
	}

	result.FileAssetURL, err = reminderRelatedAssetURL(record)

	return *result, err
}

func reminderRelatedAssetURL(record cloudkit.CKRecord) (string, error) {
	if record.Fields == nil {
		return "", nil
	}

	field, exists := (*record.Fields)[protocol.RemindersRelatedFieldFileAssetValue]
	if !exists {
		return "", nil
	}

	wrapper, err := field.AsCKPassthroughField()
	if err != nil {
		return "", fmt.Errorf("decode reminder file asset: %w", err)
	}

	if wrapper.Type != string(cloudkit.ASSET) && wrapper.Type != string(cloudkit.ASSETID) {
		return "", nil
	}

	var fields map[string]json.RawMessage

	err = json.Unmarshal(wrapper.Value, &fields)
	if err != nil {
		return "", fmt.Errorf("decode reminder file asset token: %w", err)
	}

	raw := fields[protocol.RemindersCKAssetTokenDownloadURL]
	if len(raw) == 0 || string(raw) == jsonNullValue {
		return "", nil
	}

	var downloadURL string

	err = json.Unmarshal(raw, &downloadURL)
	if err != nil {
		return "", fmt.Errorf("decode reminder file asset URL: %w", err)
	}

	return downloadURL, nil
}
