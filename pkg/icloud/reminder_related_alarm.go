package icloud

import (
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func projectReminderTrigger(record cloudkit.CKRecord) (*ReminderLocationTrigger, error) {
	kind := ""

	err := reminderRelatedStrings(record, map[string]*string{protocol.RemindersRelatedFieldTypeValue: &kind})
	if err != nil || kind != protocol.RemindersLocationTriggerTypeValue {
		return nil, err
	}

	result := new(ReminderLocationTrigger)
	result.ID = record.RecordName
	result.RecordChangeTag = reminderRelatedTag(record)

	result.AlarmID, err = reminderReference(record, protocol.RemindersRelatedFieldAlarmValue)
	if err != nil {
		return nil, err
	}

	err = reminderRelatedStrings(record, map[string]*string{
		protocol.RemindersRelatedFieldTitleValue:       &result.Title,
		protocol.RemindersRelatedFieldAddressValue:     &result.Address,
		protocol.RemindersRelatedFieldLocationUIDValue: &result.LocationUID})
	if err != nil {
		return nil, err
	}

	for name, destination := range map[string]*float64{
		protocol.RemindersRelatedFieldLatitudeValue:  &result.Latitude,
		protocol.RemindersRelatedFieldLongitudeValue: &result.Longitude,
		protocol.RemindersRelatedFieldRadiusValue:    &result.Radius} {
		*destination, err = reminderRelatedFloat(record, name)
		if err != nil {
			return nil, err
		}
	}

	proximity, err := reminderRelatedInteger(record, protocol.RemindersRelatedFieldProximityValue, int64(ReminderArriving))
	if err != nil || result.Radius < 0 {
		return nil, errReminderList
	}

	result.Proximity = ReminderLocationTriggerProximity(proximity)
	if !result.Proximity.Valid() {
		result.Proximity = ReminderArriving
	}

	return result, nil
}

func projectReminderRecurrence(record cloudkit.CKRecord) (ReminderRecurrenceRule, error) {
	result := new(ReminderRecurrenceRule)
	result.ID = record.RecordName
	result.RecordChangeTag = reminderRelatedTag(record)

	var err error

	result.ReminderID, err = reminderReference(record, protocol.RemindersRelatedFieldReminderValue)
	if err != nil {
		return *result, err
	}

	frequency, err := reminderRelatedInteger(record, protocol.RemindersRelatedFieldFrequencyValue, int64(ReminderDaily))
	if err != nil {
		return *result, err
	}

	result.Frequency = ReminderRecurrenceRuleFrequency(frequency)
	if !result.Frequency.Valid() {
		result.Frequency = ReminderDaily
	}

	result.Interval, err = reminderRelatedInteger(record, protocol.RemindersRelatedFieldIntervalValue, 1)
	if err != nil {
		return *result, err
	}

	err = projectReminderRecurrenceCounts(result, record)

	return *result, err
}

func projectReminderRecurrenceCounts(result *ReminderRecurrenceRule, record cloudkit.CKRecord) error {
	for name, destination := range map[string]*int64{
		protocol.RemindersRelatedFieldOccurrenceCountValue:   &result.OccurrenceCount,
		protocol.RemindersRelatedFieldFirstDayOfTheWeekValue: &result.FirstDayOfWeek} {
		value, err := reminderRelatedInteger(record, name, 0)
		if err != nil {
			return err
		}

		*destination = value
	}

	if result.Interval < 1 || result.OccurrenceCount < 0 || result.FirstDayOfWeek < 0 || result.FirstDayOfWeek > 6 {
		return errReminderList
	}

	return nil
}
