package webtransport

import (
	"encoding/json"
	"fmt"
	"reflect"

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
	best := -1

	record, exact, recordErr := decodeReminderNormalRecord(fields)
	if recordErr == nil {
		result.Record = &record
		best = reminderUnionRank(reflect.TypeOf(record), raw, exact)
	}

	tombstone, tombstoneErr := decodeReminderTombstone(fields)
	if tombstoneErr == nil {
		score := reminderUnionRank(reflect.TypeOf(tombstone), raw, true)
		if score > best {
			result = &ReminderEventRecord{Record: nil, Tombstone: &tombstone, Failure: nil}
			best = score
		}
	}

	if reminderSyncErrorRecord(item, fields) {
		failure, _ := item.AsCKErrorItem()

		score := reminderUnionRank(reflect.TypeOf(failure), raw, true)
		if score > best {
			result = &ReminderEventRecord{Record: nil, Tombstone: nil, Failure: &failure}
			best = score
		}
	}

	if best >= 0 {
		return result, nil
	}

	return nil, errReminderZonesShape
}
