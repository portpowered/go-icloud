package icloud

import (
	"context"
	"github.com/portpowered/go-icloud/internal/protocol"
)

// ListReminderAlarms reads alarms and joins the last supported trigger for each identifier.
func (sdk *SDK) ListReminderAlarms(ctx context.Context,
	request ListReminderAlarmsRequest) (*ListReminderAlarmsResult, error) {
	read, err := sdk.relatedRead(request.Auth, "ListReminderAlarms")
	if err != nil {
		return nil, err
	}

	records, err := read.records(ctx, request.IDs,
		protocol.RemindersAlarmIDPrefixValue, protocol.RemindersAlarmRecordTypeValue)
	if err != nil {
		return nil, read.failure(err)
	}

	result := &ListReminderAlarmsResult{Items: []ReminderAlarmWithTrigger{}, Responses: nil}
	ids := []string{}

	for _, record := range records {
		alarm, projectErr := projectReminderAlarm(record)
		if projectErr != nil {
			return nil, read.failure(projectErr)
		}

		item := ReminderAlarmWithTrigger{Alarm: alarm, Trigger: nil}
		item.Trigger.SetNull()

		result.Items = append(result.Items, item)

		if alarm.TriggerID != "" {
			ids = append(ids, alarm.TriggerID)
		}
	}

	err = read.triggers(ctx, result.Items, ids)
	if err != nil {
		return nil, read.failure(err)
	}

	result.Responses = read.metadata()

	return result, nil
}

func (read *reminderRelatedRead) triggers(ctx context.Context, items []ReminderAlarmWithTrigger, ids []string) error {
	records, err := read.records(ctx, ids,
		protocol.RemindersAlarmTriggerIDPrefixValue, protocol.RemindersAlarmTriggerRecordTypeValue)
	if err != nil {
		return err
	}

	triggers := map[string]ReminderLocationTrigger{}

	for _, record := range records {
		trigger, projectErr := projectReminderTrigger(record)
		if projectErr != nil {
			return projectErr
		}

		if trigger != nil {
			name := reminderRelatedNames([]string{trigger.ID}, protocol.RemindersAlarmTriggerIDPrefixValue)[0]
			triggers[name] = *trigger
		}
	}

	for index := range items {
		if items[index].Alarm.TriggerID == "" {
			continue
		}

		name := reminderRelatedNames([]string{items[index].Alarm.TriggerID}, protocol.RemindersAlarmTriggerIDPrefixValue)[0]
		if trigger, exists := triggers[name]; exists {
			items[index].Trigger.Set(trigger)
		}
	}

	return nil
}
