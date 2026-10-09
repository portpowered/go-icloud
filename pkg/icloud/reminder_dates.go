package icloud

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const (
	reminderMinimumMillis int64 = -62135596800000
	reminderMaximumYear   int   = 9999
)

func projectReminderDates(result *Reminder, record cloudkit.CKRecord) error {
	for name, destination := range map[string]*nullable.Nullable[time.Time]{
		protocol.RemindersReminderFieldCompletionDateValue:   &result.CompletedDate,
		protocol.RemindersReminderFieldDueDateValue:          &result.DueDate,
		protocol.RemindersReminderFieldStartDateValue:        &result.StartDate,
		protocol.RemindersReminderFieldCreationDateValue:     &result.Created,
		protocol.RemindersReminderFieldLastModifiedDateValue: &result.Modified} {
		raw, err := reminderField(record, name)
		if err != nil {
			return err
		}

		destination.SetNull()

		instant := reminderDate(raw)
		if instant == nil {
			instant = reminderAuditDate(record, name)
		}

		if instant != nil {
			destination.Set(*instant)
		}
	}

	return nil
}

func reminderDate(raw json.RawMessage) *time.Time {
	if len(raw) == 0 || string(raw) == jsonNullValue {
		return nil
	}

	millis, err := reminderInteger(raw)
	if err != nil || millis <= reminderMinimumMillis {
		return nil
	}

	instant := time.UnixMilli(millis).UTC()
	if instant.Year() > reminderMaximumYear {
		return nil
	}

	return &instant
}

func reminderAuditDate(record cloudkit.CKRecord, name string) *time.Time {
	var audit nullable.Nullable[cloudkit.CKAuditInfo]

	switch name {
	case protocol.RemindersReminderFieldCreationDateValue:
		audit = record.Created
	case protocol.RemindersReminderFieldLastModifiedDateValue:
		audit = record.Modified
	default:
		return nil
	}

	if !audit.IsSpecified() || audit.IsNull() {
		return nil
	}

	raw := json.RawMessage(strconv.FormatInt(audit.GetOrEmpty().Timestamp, 10))

	return reminderDate(raw)
}

func reminderInteger(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 || string(raw) == jsonNullValue || string(raw) == jsonFalseValue {
		return 0, nil
	}

	if string(raw) == jsonTrueValue {
		return 1, nil
	}

	var integer int64
	if json.Unmarshal(raw, &integer) == nil {
		return integer, nil
	}

	var text string
	if json.Unmarshal(raw, &text) == nil {
		integer, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("decode reminder integer: %w", err)
		}

		return integer, nil
	}

	return reminderFloatInteger(raw)
}

func reminderFloatInteger(raw json.RawMessage) (int64, error) {
	number, err := strconv.ParseFloat(string(raw), 64)
	if err != nil || math.IsInf(number, 0) || number >= float64(math.MaxInt64) || number < float64(math.MinInt64) {
		return 0, errReminderList
	}

	return int64(number), nil
}

func reminderWireInteger(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 || string(raw) == jsonNullValue {
		return 0, errReminderList
	}

	if string(raw) == jsonTrueValue || string(raw) == jsonFalseValue {
		return reminderInteger(raw)
	}

	text := string(raw)

	var decoded string

	if json.Unmarshal(raw, &decoded) == nil {
		text = strings.TrimSpace(decoded)
	}

	integer, err := strconv.ParseInt(text, 10, 64)
	if err == nil {
		return integer, nil
	}

	return reminderWireFloatInteger(text)
}

func reminderWireFloatInteger(text string) (int64, error) {
	number, err := strconv.ParseFloat(text, 64)
	if err != nil || math.Trunc(number) != number || number >= float64(math.MaxInt64) || number < float64(math.MinInt64) {
		return 0, errReminderList
	}

	return int64(number), nil
}
