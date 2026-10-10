package icloud

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/internal/reminderstext"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"golang.org/x/text/encoding/unicode"
)

func projectReminder(record cloudkit.CKRecord) (Reminder, error) {
	result := Reminder{ID: record.RecordName, ListID: "", Title: protocol.RemindersUntitledReminderValue,
		Description: "", Completed: false, CompletedDate: nil, DueDate: nil, StartDate: nil,
		Priority: 0, Flagged: false, AllDay: false, Deleted: false, TimeZone: nil,
		AlarmIDs: []string{}, HashtagIDs: []string{}, AttachmentIDs: []string{}, RecurrenceRuleIDs: []string{},
		ParentReminderID: nil, Created: nil, Modified: nil, RecordChangeTag: record.RecordChangeTag}
	if !result.RecordChangeTag.IsSpecified() {
		result.RecordChangeTag.SetNull()
	}

	var err error

	result.ListID, err = reminderReference(record, protocol.RemindersReminderFieldListValue)
	if err != nil {
		return result, err
	}

	result.Title = reminderDocumentText(record, protocol.RemindersReminderFieldTitleDocumentValue,
		protocol.RemindersUntitledReminderValue, protocol.RemindersUnreadableReminderTitleValue)
	result.Description = reminderDocumentText(record, protocol.RemindersReminderFieldNotesDocumentValue, "", "")

	err = projectReminderFlags(&result, record)
	if err != nil {
		return result, err
	}

	err = projectReminderRelations(&result, record)
	if err != nil {
		return result, err
	}

	err = projectReminderDates(&result, record)
	if err != nil {
		return result, err
	}

	return result, nil
}

func reminderDocumentText(record cloudkit.CKRecord, name, absent, unreadable string) string {
	raw, err := reminderField(record, name)
	if err != nil {
		return unreadable
	}

	present, err := reminderTruthy(raw)
	if err != nil {
		return unreadable
	}

	if !present {
		return absent
	}

	var encoded string

	err = json.Unmarshal(raw, &encoded)
	if err != nil {
		return unreadable
	}

	bytes, err := reminderRelatedBytes(record, name)
	if err != nil {
		return unreadable
	}

	if !bytes {
		if strings.ContainsFunc(encoded, func(character rune) bool { return character >= utf8.RuneSelf }) {
			return unreadable
		}

		padding := (reminderBase64Quantum - len(encoded)%reminderBase64Quantum) % reminderBase64Quantum
		encoded += strings.Repeat("=", padding)
	}

	data, err := reminderRelatedBase64(encoded)
	if err != nil {
		return unreadable
	}

	if bytes && len(data) == 0 {
		return absent
	}

	text, err := reminderstext.Decode(data)
	if err != nil {
		return unreadable
	}
	// Replace each ill-formed UTF-8 subsequence as the Source bytes decoder does.
	decoded, err := unicode.UTF8.NewDecoder().String(text)
	if err != nil {
		return unreadable
	}

	return decoded
}

func reminderReference(record cloudkit.CKRecord, name string) (string, error) {
	raw, err := reminderReferenceValue(record, name)
	if err != nil {
		return "", err
	}

	if len(raw) == 0 || string(raw) == jsonNullValue {
		return "", nil
	}

	var reference cloudkit.CKReference

	err = json.Unmarshal(raw, &reference)
	if err != nil {
		return "", fmt.Errorf("decode reminder reference: %w", err)
	}

	return reference.RecordName, nil
}

func reminderReferenceValue(record cloudkit.CKRecord, name string) (json.RawMessage, error) {
	if record.Fields == nil {
		return nil, nil
	}

	field, exists := (*record.Fields)[name]
	if !exists {
		return nil, nil
	}

	wrapper, err := field.AsCKPassthroughField()
	if err != nil {
		return nil, fmt.Errorf("decode reminder reference wrapper: %w", err)
	}

	if wrapper.Type != string(cloudkit.CKReferenceFieldTypeREFERENCE) {
		return nil, nil
	}

	return wrapper.Value, nil
}

func projectReminderFlags(result *Reminder, record cloudkit.CKRecord) error {
	for name, destination := range map[string]*bool{
		protocol.RemindersReminderFieldCompletedValue: &result.Completed,
		protocol.RemindersReminderFieldFlaggedValue:   &result.Flagged,
		protocol.RemindersReminderFieldAllDayValue:    &result.AllDay,
		protocol.RemindersReminderFieldDeletedValue:   &result.Deleted} {
		raw, err := reminderField(record, name)
		if err != nil {
			return err
		}

		*destination, err = reminderTruthy(raw)
		if err != nil {
			return err
		}
	}

	raw, err := reminderField(record, protocol.RemindersReminderFieldPriorityValue)
	if err != nil {
		return err
	}

	result.Priority, err = reminderInteger(raw)

	return err
}
