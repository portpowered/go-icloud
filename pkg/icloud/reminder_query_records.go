package icloud

import (
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func (read *reminderQueryRead) ingest(record cloudkit.CKRecord) error {
	switch record.RecordType {
	case protocol.RemindersReminderRecordTypeValue:
		return read.ingestReminder(record)
	case protocol.RemindersAlarmRecordTypeValue:
		return ingestReminderRelated(read.result.Alarms, record, projectReminderAlarm)
	case protocol.RemindersAlarmTriggerRecordTypeValue:
		return read.ingestTrigger(record)
	case protocol.RemindersAttachmentRecordTypeValue:
		return read.ingestAttachment(record)
	case protocol.RemindersHashtagRecordTypeValue:
		return ingestReminderRelated(read.result.Hashtags, record, projectReminderHashtag)
	case protocol.RemindersRecurrenceRuleRecordTypeValue:
		return ingestReminderRelated(read.result.RecurrenceRules, record, projectReminderRecurrence)
	default:
		return nil
	}
}

func (read *reminderQueryRead) ingestReminder(record cloudkit.CKRecord) error {
	value, err := projectReminder(record)
	if err != nil {
		return err
	}

	position, exists := read.positions[value.ID]
	if exists {
		read.result.Reminders[position] = value
	} else {
		read.positions[value.ID] = len(read.result.Reminders)
		read.result.Reminders = append(read.result.Reminders, value)
	}

	return nil
}

func (read *reminderQueryRead) ingestTrigger(record cloudkit.CKRecord) error {
	value, err := projectReminderTrigger(record)
	if err != nil {
		return err
	}

	if value != nil {
		read.result.Triggers[value.ID] = *value
	}

	return nil
}

func (read *reminderQueryRead) ingestAttachment(record cloudkit.CKRecord) error {
	value, err := projectReminderAttachment(record)
	if err != nil {
		return err
	}

	if value != nil {
		read.result.Attachments[record.RecordName] = *value
	}

	return nil
}

func ingestReminderRelated[T any](records map[string]T, record cloudkit.CKRecord,
	project func(cloudkit.CKRecord) (T, error),
) error {
	value, err := project(record)
	if err != nil {
		return err
	}

	records[record.RecordName] = value

	return nil
}

func scopeReminderQuery(result *ListRemindersResult, listID string) error {
	reminders := make([]Reminder, 0, len(result.Reminders))
	ids := make(map[string]bool)

	for _, reminder := range result.Reminders {
		if reminder.ListID == listID {
			reminders = append(reminders, reminder)
			ids[reminder.ID] = true
		}
	}

	result.Reminders = reminders
	scopeReminderRelated(result.Alarms, ids, func(value ReminderAlarm) string { return value.ReminderID })
	scopeReminderRelated(result.Hashtags, ids, func(value ReminderHashtag) string { return value.ReminderID })
	scopeReminderRelated(result.RecurrenceRules, ids,
		func(value ReminderRecurrenceRule) string { return value.ReminderID })

	alarmIDs := make(map[string]bool)
	for id := range result.Alarms {
		alarmIDs[id] = true
	}

	scopeReminderRelated(result.Triggers, alarmIDs, func(value ReminderLocationTrigger) string { return value.AlarmID })

	for identifier, attachment := range result.Attachments {
		// Both generated alternatives carry the same reminder identity member.
		identity, err := attachment.AsReminderURLAttachment()
		if err != nil {
			return err
		}

		if !ids[identity.ReminderID] {
			delete(result.Attachments, identifier)
		}
	}

	return nil
}

func scopeReminderRelated[T any](records map[string]T, ids map[string]bool, relation func(T) string) {
	for id, record := range records {
		if !ids[relation(record)] {
			delete(records, id)
		}
	}
}
