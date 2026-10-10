package webtransport

import (
	"encoding/json"
	"fmt"
	"maps"

	"github.com/portpowered/go-icloud/internal/protocol"
)

func normalizeReminderMetadata(fields map[string]json.RawMessage) error {
	for _, name := range []string{protocol.RemindersCKRecordCreated, protocol.RemindersCKRecordModified} {
		err := normalizeReminderAudit(fields, name)
		if err != nil {
			return err
		}
	}

	for _, name := range []string{protocol.RemindersCKRecordOwner, protocol.RemindersCKRecordCurrentUserParticipant} {
		raw, supplied := fields[name]
		if !supplied || string(raw) == jsonNullValue {
			continue
		}

		value, err := normalizeReminderParticipant(raw)
		if err != nil {
			return err
		}

		fields[name] = value
	}

	return normalizeReminderParticipantArrays(fields)
}

func normalizeReminderParticipantArrays(fields map[string]json.RawMessage) error {
	for _, name := range []string{protocol.RemindersCKRecordParticipants,
		protocol.RemindersCKRecordRequesters, protocol.RemindersCKRecordBlocked} {
		raw, supplied := fields[name]
		if !supplied || string(raw) == jsonNullValue {
			continue
		}

		var values []json.RawMessage

		err := json.Unmarshal(raw, &values)
		if err != nil {
			return fmt.Errorf("decode reminder participant array: %w", err)
		}

		for index, value := range values {
			values[index], err = normalizeReminderParticipant(value)
			if err != nil {
				return err
			}
		}

		fields[name], err = json.Marshal(values)
		if err != nil {
			return fmt.Errorf("encode reminder participant array: %w", err)
		}
	}

	return nil
}

func normalizeReminderParticipant(raw json.RawMessage) (json.RawMessage, error) {
	fields, err := accountFields(raw)
	if err != nil || string(raw) == jsonNullValue {
		return nil, errReminderZonesShape
	}

	normalized := maps.Clone(fields)

	err = normalizeReminderParticipantBooleans(normalized)
	if err != nil {
		return nil, err
	}

	for _, name := range []string{protocol.RemindersCKParticipantPublicKeyVersion,
		protocol.RemindersCKParticipantOutOfNetworkKeyType} {
		value, supplied := normalized[name]
		if !supplied || string(value) == jsonNullValue {
			continue
		}

		normalized[name], err = reminderIntegerJSON(value)
		if err != nil {
			return nil, err
		}
	}

	body, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("encode reminder participant: %w", err)
	}

	return body, nil
}

func normalizeReminderParticipantBooleans(normalized map[string]json.RawMessage) error {
	for _, name := range []string{protocol.RemindersCKParticipantIsApprovedRequester,
		protocol.RemindersCKParticipantOrgUser} {
		value, supplied := normalized[name]
		if !supplied || string(value) == jsonNullValue {
			continue
		}

		if !reminderSyncBoolean(value) {
			return errReminderZonesShape
		}

		normalized[name] = reminderBooleanJSON(value)
	}

	return nil
}

func reminderBooleanJSON(raw json.RawMessage) json.RawMessage {
	if reminderEncryptedTrue(raw) {
		return json.RawMessage(jsonTrueValue)
	}

	return json.RawMessage(jsonFalseValue)
}
