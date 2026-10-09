package icloud

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func projectReminderRelations(result *Reminder, record cloudkit.CKRecord) error {
	for name, destination := range map[string]*[]string{
		protocol.RemindersReminderFieldAlarmIDsValue:          &result.AlarmIDs,
		protocol.RemindersReminderFieldHashtagIDsValue:        &result.HashtagIDs,
		protocol.RemindersReminderFieldAttachmentIDsValue:     &result.AttachmentIDs,
		protocol.RemindersReminderFieldRecurrenceRuleIDsValue: &result.RecurrenceRuleIDs} {
		raw, err := reminderField(record, name)
		if err != nil {
			return err
		}

		present, err := reminderTruthy(raw)
		if err != nil {
			return err
		}

		if !present {
			continue
		}

		var values []string

		err = json.Unmarshal(raw, &values)
		if err != nil {
			return fmt.Errorf("decode reminder related IDs: %w", err)
		}

		for _, value := range values {
			*destination = append(*destination, strings.TrimPrefix(value, reminderRelationPrefix(name)))
		}
	}

	parent, err := reminderReference(record, protocol.RemindersReminderFieldParentReminderValue)
	if err != nil {
		return err
	}

	result.ParentReminderID.SetNull()

	if parent != "" {
		result.ParentReminderID.Set(parent)
	}

	return projectReminderString(record, protocol.RemindersReminderFieldTimeZoneValue, &result.TimeZone)
}

func reminderRelationPrefix(name string) string {
	switch name {
	case protocol.RemindersReminderFieldAlarmIDsValue:
		return protocol.RemindersAlarmIDPrefixValue
	case protocol.RemindersReminderFieldHashtagIDsValue:
		return protocol.RemindersHashtagIDPrefixValue
	case protocol.RemindersReminderFieldAttachmentIDsValue:
		return protocol.RemindersAttachmentIDPrefixValue
	default:
		return protocol.RemindersRecurrenceRuleIDPrefixValue
	}
}
