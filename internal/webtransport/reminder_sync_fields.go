package webtransport

import (
	"encoding/json"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func validateReminderSyncFields(fields *map[string]cloudkit.CKFieldOpen) bool {
	if fields == nil {
		return true
	}

	for _, field := range *fields {
		if !validReminderSyncField(field) {
			return false
		}
	}

	return true
}

func validReminderSyncField(field cloudkit.CKFieldOpen) bool {
	if !validReminderSyncTag(field) {
		return false
	}

	value, err := field.AsCKPassthroughField()
	if err != nil {
		// Source wraps untagged primitive values as unknown fields.
		return true
	}

	if !validReminderSyncEncryption(value) {
		return false
	}

	validators := map[string]func(json.RawMessage) bool{
		string(cloudkit.TIMESTAMP):                             reminderSyncTimestamp,
		string(cloudkit.CKInt64FieldTypeINT64):                 reminderSyncInteger,
		string(cloudkit.ENCRYPTEDBYTES):                        reminderSyncBytes,
		string(cloudkit.BYTES):                                 reminderSyncBytes,
		string(cloudkit.CKReferenceFieldTypeREFERENCE):         reminderSyncReference,
		string(cloudkit.CKReferenceListFieldTypeREFERENCELIST): reminderSyncReferences,
		string(cloudkit.CKStringFieldTypeSTRING):               reminderSyncString,
		string(cloudkit.CKStringListFieldTypeSTRINGLIST):       reminderSyncStrings,
		string(cloudkit.ASSETID):                               reminderSyncAsset,
		string(cloudkit.ASSET):                                 reminderSyncAsset,
		string(cloudkit.ASSETIDLIST):                           reminderSyncAssets,
		string(cloudkit.DOUBLE):                                reminderSyncDouble,
		string(cloudkit.DOUBLELIST):                            reminderSyncDoubles,
		string(cloudkit.INT64LIST):                             reminderSyncIntegers,
		string(cloudkit.UNKNOWNLIST):                           reminderSyncUnknownList,
	}

	validate, known := validators[value.Type]
	if !known {
		return true
	}

	return len(value.Value) != 0 && validate(value.Value)
}

func validReminderSyncTag(field cloudkit.CKFieldOpen) bool {
	raw, err := json.Marshal(field)
	if err != nil {
		return false
	}

	fields, err := accountFields(raw)
	if err != nil {
		return true
	}

	if _, exists := fields[protocol.RemindersCKPassthroughFieldValue]; !exists {
		return true
	}

	tag, exists := fields[protocol.RemindersCKPassthroughFieldType]
	if !exists {
		return true
	}

	var name string

	return string(tag) != jsonNullValue && json.Unmarshal(tag, &name) == nil
}

func validReminderSyncEncryption(field cloudkit.CKPassthroughField) bool {
	if field.Type != string(cloudkit.CKStringFieldTypeSTRING) && field.Type != string(cloudkit.DOUBLE) {
		return true
	}

	raw, exists := field.AdditionalProperties[protocol.RemindersCKStringFieldIsEncrypted]

	return !exists || reminderSyncBoolean(raw)
}

func reminderSyncString(raw json.RawMessage) bool {
	if string(raw) == jsonNullValue {
		return true
	}

	var value string

	return json.Unmarshal(raw, &value) == nil
}

func reminderSyncReference(raw json.RawMessage) bool {
	if string(raw) == jsonNullValue {
		return true
	}

	fields, err := accountFields(raw)
	if err != nil || !reminderRequiredValue(fields, protocol.RemindersCKReferenceRecordName) {
		return false
	}

	if zone, exists := fields[protocol.RemindersCKReferenceZoneID]; exists && string(zone) != jsonNullValue {
		identity, zoneErr := accountFields(zone)
		if zoneErr != nil || !reminderRequiredValue(identity, protocol.RemindersCKZoneIDZoneName) {
			return false
		}
	}

	var reference cloudkit.CKReference

	return json.Unmarshal(raw, &reference) == nil
}

func reminderSyncAsset(raw json.RawMessage) bool {
	fields, err := accountFields(raw)
	if err != nil {
		return false
	}

	if data, exists := fields[protocol.RemindersCKAssetTokenDownloadedData]; exists && string(data) != jsonNullValue {
		if !reminderSyncBytes(data) {
			return false
		}
	}

	if size, exists := fields[protocol.RemindersCKAssetTokenSize]; exists && string(size) != jsonNullValue {
		if !reminderSyncInteger(size) {
			return false
		}

		delete(fields, protocol.RemindersCKAssetTokenSize)
	}

	encoded, err := json.Marshal(fields)
	if err != nil {
		return false
	}

	var asset cloudkit.CKAssetToken

	return json.Unmarshal(encoded, &asset) == nil
}
