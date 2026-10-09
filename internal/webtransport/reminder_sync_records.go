package webtransport

import (
	"encoding/json"
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func validateReminderSyncRecords(records *[]cloudkit.CKQueryResponse_Records_Item) error {
	if records == nil {
		return nil
	}

	for _, item := range *records {
		raw, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("encode reminder sync record: %w", err)
		}

		fields, err := accountFields(raw)
		if err != nil {
			return err
		}

		if reminderSyncNormalRecord(item, fields) || reminderSyncErrorRecord(item, fields) ||
			reminderSyncTombstone(item, fields) {
			continue
		}

		return errReminderZonesShape
	}

	return nil
}

func reminderSyncNormalRecord(item cloudkit.CKQueryResponse_Records_Item,
	fields map[string]json.RawMessage,
) bool {
	if !reminderRequiredValue(fields, protocol.RemindersCKRecordRecordName) ||
		!reminderRequiredValue(fields, protocol.RemindersCKRecordRecordType) {
		return false
	}

	record, err := item.AsCKRecord()
	if err != nil {
		return false
	}

	return validateReminderSyncFields(record.Fields)
}

func reminderSyncErrorRecord(item cloudkit.CKQueryResponse_Records_Item,
	fields map[string]json.RawMessage,
) bool {
	if !reminderRequiredValue(fields, protocol.RemindersCKErrorItemServerErrorCode) {
		return false
	}

	_, err := item.AsCKErrorItem()

	return err == nil
}

func reminderSyncTombstone(item cloudkit.CKQueryResponse_Records_Item,
	fields map[string]json.RawMessage,
) bool {
	if !reminderRequiredValue(fields, protocol.RemindersCKTombstoneRecordRecordName) ||
		string(fields[protocol.RemindersCKTombstoneRecordDeleted]) != "true" {
		return false
	}

	_, err := item.AsCKTombstoneRecord()

	return err == nil
}
