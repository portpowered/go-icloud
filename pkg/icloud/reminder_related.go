package icloud

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const reminderBase64Quantum = 4

func reminderRelatedTag(record cloudkit.CKRecord) nullable.Nullable[string] {
	tag := record.RecordChangeTag
	if !tag.IsSpecified() {
		tag.SetNull()
	}

	return tag
}

func reminderRelatedStrings(record cloudkit.CKRecord, destinations map[string]*string) error {
	for name, destination := range destinations {
		raw, err := reminderRelatedText(record, name, false)
		if err != nil {
			return err
		}

		present, err := reminderTruthy(raw)
		if err != nil {
			return err
		}

		if present {
			err = json.Unmarshal(raw, destination)
			if err != nil {
				return fmt.Errorf("decode related reminder string: %w", err)
			}
		}
	}

	return nil
}

func reminderRelatedInteger(record cloudkit.CKRecord, name string, fallback int64) (int64, error) {
	raw, err := reminderField(record, name)
	if err != nil {
		return 0, err
	}

	present, err := reminderTruthy(raw)
	if err != nil {
		return 0, err
	}

	if !present {
		return fallback, nil
	}

	return reminderInteger(raw)
}

func reminderRelatedFloat(record cloudkit.CKRecord, name string) (float64, error) {
	raw, err := reminderField(record, name)
	if err != nil {
		return 0, err
	}

	present, err := reminderTruthy(raw)
	if err != nil || !present {
		return 0, err
	}

	var text string
	if json.Unmarshal(raw, &text) != nil {
		text = string(raw)
	}

	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, fmt.Errorf("decode related reminder coordinate: %w", err)
	}

	return value, nil
}

func projectReminderAlarm(record cloudkit.CKRecord) (ReminderAlarm, error) {
	result := new(ReminderAlarm)
	result.ID = record.RecordName
	result.RecordChangeTag = reminderRelatedTag(record)

	var err error

	result.ReminderID, err = reminderReference(record, protocol.RemindersRelatedFieldReminderValue)
	if err != nil {
		return *result, err
	}

	err = reminderRelatedStrings(record, map[string]*string{
		protocol.RemindersRelatedFieldAlarmUIDValue:  &result.AlarmUID,
		protocol.RemindersRelatedFieldTriggerIDValue: &result.TriggerID})

	return *result, err
}

func projectReminderHashtag(record cloudkit.CKRecord) (ReminderHashtag, error) {
	result := new(ReminderHashtag)
	result.ID = record.RecordName
	result.RecordChangeTag = reminderRelatedTag(record)
	result.Created.SetNull()

	var err error

	result.ReminderID, err = reminderReference(record, protocol.RemindersRelatedFieldReminderValue)
	if err != nil {
		return *result, err
	}

	raw, err := reminderRelatedText(record, protocol.RemindersRelatedFieldNameValue, true)
	if err != nil {
		return *result, err
	}

	if len(raw) != 0 && string(raw) != jsonNullValue {
		result.Name, err = reminderDisplayText(raw)
		if err != nil {
			return *result, err
		}
	}

	raw, err = reminderField(record, protocol.RemindersReminderFieldCreationDateValue)
	if err != nil {
		return *result, err
	}

	if instant := reminderDate(raw); instant != nil {
		result.Created.Set(*instant)
	}

	return *result, nil
}

func reminderAttachmentURL(value string) string {
	if reminderLooksLikeURL(value) {
		return value
	}

	// Go's decoder ignores CR and LF; Source's strict URL decoder rejects them.
	if strings.ContainsAny(value, "\r\n") {
		return value
	}

	padding := (reminderBase64Quantum - len(value)%reminderBase64Quantum) % reminderBase64Quantum

	decoded, err := base64.StdEncoding.DecodeString(value + strings.Repeat("=", padding))
	if err == nil && utf8.Valid(decoded) && reminderLooksLikeURL(string(decoded)) {
		return string(decoded)
	}

	return value
}

func reminderLooksLikeURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" {
		return false
	}

	if parsed.Scheme == "http" || parsed.Scheme == "https" {
		return parsed.Host != ""
	}

	return parsed.Host != "" || parsed.Path != "" || parsed.Opaque != ""
}
