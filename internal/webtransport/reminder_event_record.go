package webtransport

import (
	"encoding/json"
	"fmt"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// ReminderEventRecord identifies the validated alternative in a wire record union.
type ReminderEventRecord struct {
	// Record is a normal record with valid identity and field wrappers.
	Record *cloudkit.CKRecord
	// Tombstone is a deleted record without a reminder snapshot.
	Tombstone *cloudkit.CKTombstoneRecord
	// Failure is a per-record provider error.
	Failure *cloudkit.CKErrorItem
}

// DecodeReminderEventRecord selects a valid generated model alternative.
func DecodeReminderEventRecord(item cloudkit.CKZoneChangesZone_Records_Item) (*ReminderEventRecord, error) {
	raw, err := json.Marshal(item)
	if err != nil {
		return nil, fmt.Errorf("encode reminder event record: %w", err)
	}

	fields, err := accountFields(raw)
	if err != nil {
		return nil, err
	}

	result := new(ReminderEventRecord)

	if reminderSyncNormalRecord(item, fields) {
		record, _ := item.AsCKRecord()
		result.Record = &record

		return result, nil
	}

	if reminderSyncTombstone(item, fields) {
		tombstone, _ := item.AsCKTombstoneRecord()
		result.Tombstone = &tombstone

		return result, nil
	}

	if reminderSyncErrorRecord(item, fields) {
		failure, _ := item.AsCKErrorItem()
		result.Failure = &failure

		return result, nil
	}

	return nil, errReminderZonesShape
}
