package webtransport

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func decodeReminderNormalRecord(fields map[string]json.RawMessage) (cloudkit.CKRecord, bool, error) {
	var record cloudkit.CKRecord

	if !reminderModelRequired(reflect.TypeFor[cloudkit.CKRecord](), fields) {
		return record, false, errReminderZonesShape
	}

	normalized, exact, err := normalizeReminderRecordBooleans(fields)
	if err != nil {
		return record, false, err
	}

	body, err := json.Marshal(normalized)
	if err != nil {
		return record, false, fmt.Errorf("encode normalized reminder record: %w", err)
	}

	err = json.Unmarshal(body, &record)
	if err != nil {
		return record, false, fmt.Errorf("decode normalized reminder record: %w", err)
	}

	if !validateReminderSyncFields(record.Fields) {
		return record, false, errReminderZonesShape
	}

	return record, exact, nil
}

func normalizeReminderRecordBooleans(fields map[string]json.RawMessage) (map[string]json.RawMessage, bool, error) {
	normalized := maps.Clone(fields)
	exact := true

	for _, name := range []string{protocol.RemindersCKRecordDeleted, protocol.RemindersCKRecordDenyAccessRequests} {
		raw, supplied := normalized[name]
		if !supplied || string(raw) == jsonNullValue || string(raw) == jsonTrueValue || string(raw) == jsonFalseValue {
			continue
		}

		if !reminderSyncBoolean(raw) {
			return nil, false, errReminderZonesShape
		}

		exact = false

		normalized[name] = json.RawMessage(jsonFalseValue)
		if reminderEncryptedTrue(raw) {
			normalized[name] = json.RawMessage(jsonTrueValue)
		}
	}

	return normalized, exact, nil
}

func decodeReminderTombstone(fields map[string]json.RawMessage) (cloudkit.CKTombstoneRecord, error) {
	var record cloudkit.CKTombstoneRecord

	if !reminderModelRequired(reflect.TypeFor[cloudkit.CKTombstoneRecord](), fields) ||
		!reminderLiteralTrue(fields[protocol.RemindersCKTombstoneRecordDeleted]) {
		return record, errReminderZonesShape
	}

	normalized := maps.Clone(fields)
	normalized[protocol.RemindersCKTombstoneRecordDeleted] = json.RawMessage(jsonTrueValue)

	body, err := json.Marshal(normalized)
	if err != nil {
		return record, fmt.Errorf("encode normalized reminder tombstone: %w", err)
	}

	err = json.Unmarshal(body, &record)
	if err != nil {
		return record, fmt.Errorf("decode normalized reminder tombstone: %w", err)
	}

	return record, nil
}

func reminderLiteralTrue(raw json.RawMessage) bool {
	if string(raw) == jsonTrueValue {
		return true
	}

	var number float64

	return json.Unmarshal(raw, &number) == nil && number == 1
}
