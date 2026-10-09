package icloud

import (
	"encoding/json"
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func reminderRelatedFrequency(record cloudkit.CKRecord) (ReminderRecurrenceRuleFrequency, error) {
	raw, err := reminderField(record, protocol.RemindersRelatedFieldFrequencyValue)
	if err != nil {
		return ReminderDaily, err
	}

	if len(raw) == 0 || string(raw) == jsonNullValue || string(raw) == jsonTrueValue {
		return ReminderDaily, nil
	}

	var number float64

	if raw[0] == '"' {
		number, err = reminderRelatedFrequencyText(record)
		if err != nil {
			return ReminderDaily, err
		}
	} else {
		// Non-numeric passthrough values cannot equal a Source IntEnum member.
		err = json.Unmarshal(raw, &number)
		if err != nil {
			number = 0
		}
	}

	for _, candidate := range []ReminderRecurrenceRuleFrequency{
		ReminderDaily, ReminderWeekly, ReminderMonthly, ReminderYearly} {
		if number == float64(candidate) {
			return candidate, nil
		}
	}

	return ReminderDaily, nil
}

func reminderRelatedFrequencyText(record cloudkit.CKRecord) (float64, error) {
	// DOUBLE wrappers coerce numeric text before Source enum construction;
	// STRING and byte wrappers keep their original value and fall back to daily.
	field := (*record.Fields)[protocol.RemindersRelatedFieldFrequencyValue]

	wrapper, err := field.AsCKPassthroughField()
	if err != nil {
		return 0, fmt.Errorf("decode recurrence frequency wrapper: %w", err)
	}

	if wrapper.Type != string(cloudkit.DOUBLE) {
		return 0, nil
	}

	return reminderRelatedFloat(record, protocol.RemindersRelatedFieldFrequencyValue)
}
