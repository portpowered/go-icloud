package webtransport

import (
	"bytes"
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func reminderRequestZone() cloudkit.CKZoneIDReq {
	zone := new(cloudkit.CKZoneIDReq)
	zone.ZoneName = protocol.RemindersZoneNameValue
	zone.ZoneType.Set(protocol.RemindersZoneTypeValue)

	return *zone
}

func reminderChangeZone() cloudkit.CKZoneID {
	zone := new(cloudkit.CKZoneID)
	zone.ZoneName = protocol.RemindersZoneNameValue
	zone.ZoneType.Set(protocol.RemindersZoneTypeValue)

	return *zone
}

func reminderChangesRequestBody(zone cloudkit.CKZoneChangesZoneReq) ([]byte, error) {
	input := new(cloudkit.CKZoneChangesRequest)
	input.Zones = []cloudkit.CKZoneChangesZoneReq{zone}

	body, err := referenceJSON(input)
	if err != nil {
		return nil, err
	}

	return reminderOrderedObject(body, zone, []string{protocol.RemindersCKZoneChangesZoneReqZoneID,
		protocol.RemindersCKZoneChangesZoneReqDesiredKeys, protocol.RemindersCKZoneChangesZoneReqDesiredRecordTypes,
		protocol.RemindersCKZoneChangesZoneReqSyncToken, protocol.RemindersCKZoneChangesZoneReqReverse})
}

// reminderOrderedObject only reorders a serialization of the same generated model.
func reminderOrderedObject(body []byte, model any, fields []string) ([]byte, error) {
	original, err := referenceJSON(model)
	if err != nil {
		return nil, fmt.Errorf("encode reminder request object: %w", err)
	}

	ordered, err := referenceJSONFields(model, fields)
	if err != nil {
		return nil, fmt.Errorf("order reminder request object: %w", err)
	}

	return bytes.ReplaceAll(body, original, ordered), nil
}
