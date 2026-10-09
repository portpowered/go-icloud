package webtransport

import (
	"encoding/json"
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

type reminderSyncRecord interface {
	json.Marshaler
	AsCKRecord() (cloudkit.CKRecord, error)
	AsCKErrorItem() (cloudkit.CKErrorItem, error)
	AsCKTombstoneRecord() (cloudkit.CKTombstoneRecord, error)
}

func validateReminderSyncRecords[T reminderSyncRecord](records *[]T) error {
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

		if reminderSyncNormalRecord(fields) || reminderSyncErrorRecord(item, fields) ||
			reminderSyncTombstone(fields) {
			continue
		}

		return errReminderZonesShape
	}

	return nil
}

func reminderSyncNormalRecord(fields map[string]json.RawMessage) bool {
	_, _, err := decodeReminderNormalRecord(fields)

	return err == nil
}

func reminderSyncErrorRecord(item reminderSyncRecord,
	fields map[string]json.RawMessage,
) bool {
	if !reminderRequiredValue(fields, protocol.RemindersCKErrorItemServerErrorCode) {
		return false
	}

	_, err := item.AsCKErrorItem()

	return err == nil
}

func reminderSyncTombstone(fields map[string]json.RawMessage) bool {
	_, err := decodeReminderTombstone(fields)

	return err == nil
}
