package icloud

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/internal/reminderstext"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func reminderWriteInteger(value int64) cloudkit.ReminderWriteInteger {
	return cloudkit.ReminderWriteInteger{Type: cloudkit.ReminderWriteIntegerTypeINT64, Value: value}
}

func reminderBoolean(value bool) cloudkit.ReminderWriteInteger {
	if value {
		return reminderWriteInteger(1)
	}

	return reminderWriteInteger(0)
}

func reminderString(value string) cloudkit.ReminderOptionalString {
	field := cloudkit.ReminderOptionalString{Type: cloudkit.ReminderOptionalSTRINGType, Value: nil}
	field.Value.Set(value)

	return field
}

func reminderTimestamp(value time.Time) cloudkit.ReminderOptionalTimestamp {
	field := cloudkit.ReminderOptionalTimestamp{Type: cloudkit.ReminderOptionalTIMESTAMPType, Value: nil}
	seconds := float64(value.Unix()) + float64(value.Nanosecond())/float64(time.Second)
	field.Value.Set(int64(seconds * float64(time.Second/time.Millisecond)))

	return field
}

func reminderMutationInstant(value time.Time) time.Time {
	return time.UnixMilli(reminderTimestamp(value).Value.GetOrEmpty()).UTC()
}

func reminderWriteReference(value string) cloudkit.ReminderWriteReference {
	field := cloudkit.ReminderWriteReference{Type: cloudkit.ReminderWriteReferenceTypeREFERENCE, Value: nil}
	if value != "" {
		field.Value = &cloudkit.ReminderWriteReferenceValue{
			RecordName: value, Action: cloudkit.ReminderWriteReferenceValueActionVALIDATE}
	}

	return field
}

func reminderWriteZone() cloudkit.CKZoneIDReq {
	zone := cloudkit.CKZoneIDReq{ZoneName: protocol.RemindersZoneNameValue,
		ZoneType: nil, OwnerRecordName: nil, AdditionalProperties: nil}
	zone.ZoneType.Set(protocol.RemindersZoneTypeValue)

	return zone
}

func reminderDocuments(titleText, notesText string) (string, string, error) {
	title, err := reminderstext.Encode(titleText)
	if err != nil {
		return "", "", fmt.Errorf("encode reminder title: %w", err)
	}

	notes, err := reminderstext.Encode(notesText)
	if err != nil {
		return "", "", fmt.Errorf("encode reminder notes: %w", err)
	}

	return title, notes, nil
}

func (sdk *SDK) reminderTokens(now time.Time, count int) ([]cloudkit.ReminderResolutionToken, error) {
	tokens := make([]cloudkit.ReminderResolutionToken, count)
	for index := range tokens {
		token, err := sdk.reminderResolutionToken(now)
		if err != nil {
			return nil, err
		}

		tokens[index] = token
	}

	return tokens, nil
}

func reminderDocument(value string) cloudkit.ReminderDocumentWriteString {
	return cloudkit.ReminderDocumentWriteString{Type: cloudkit.ReminderDocumentWriteStringTypeSTRING, Value: value}
}

func reminderTokenString(value any) (cloudkit.ReminderWriteString, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return cloudkit.ReminderWriteString{}, fmt.Errorf("encode reminder resolution tokens: %w", err)
	}

	return cloudkit.ReminderWriteString{Type: cloudkit.ReminderWriteStringTypeSTRING, Value: string(encoded)}, nil
}
