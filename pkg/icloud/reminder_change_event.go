package icloud

import (
	"errors"
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/internal/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

var errReminderChangeRecord = errors.New("provider rejected reminder change record")

func (read *reminderChangesRead) event(item cloudkit.CKZoneChangesZone_Records_Item) error {
	value, err := webtransport.DecodeReminderEventRecord(item)
	if err != nil {
		return fmt.Errorf("decode reminder change event: %w", err)
	}

	if value.Failure != nil {
		return read.responseFailure(Provider, fmt.Errorf("%w: %s", errReminderChangeRecord, value.Failure.ServerErrorCode))
	}

	event := ReminderChangeEvent{Type: ReminderChangeDeleted, ReminderID: "", Reminder: nil}
	if value.Tombstone != nil {
		event.ReminderID = value.Tombstone.RecordName
		event.Reminder.SetNull()
		read.changes = append(read.changes, event)

		return nil
	}

	if value.Record.RecordType != protocol.RemindersReminderRecordTypeValue {
		return nil
	}

	reminder, err := projectReminder(*value.Record)
	if err != nil {
		return fmt.Errorf("project reminder change: %w", err)
	}

	event.ReminderID = reminder.ID
	event.Reminder.Set(reminder)

	if !reminder.Deleted {
		event.Type = ReminderChangeUpdated
	}

	read.changes = append(read.changes, event)

	return nil
}
