package icloud

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func locationNumber(value float64) json.Number {
	// Python's shortest float spelling switches to scientific notation outside this interval.
	const (
		lowerScientificBoundary = 1e-4
		upperScientificBoundary = 1e16
	)

	format := byte('f')

	absolute := math.Abs(value)
	if absolute != 0 && (absolute < lowerScientificBoundary || absolute >= upperScientificBoundary) {
		format = 'e'
	}

	text := strconv.FormatFloat(value, format, -1, 64)
	if !strings.ContainsAny(text, ".e") {
		text += ".0"
	}

	return json.Number(text)
}

func locationString(value string) cloudkit.LocationString {
	return cloudkit.LocationString{Value: value, Type: cloudkit.LocationStringTypeLocationStringType}
}

func locationEncryptedString(value string) cloudkit.LocationEncryptedString {
	return cloudkit.LocationEncryptedString{Value: value,
		IsEncrypted: cloudkit.LocationEncryptedStringEncryptedTrue,
		Type:        cloudkit.LocationEncryptedStringTypeLocationEncryptedStringType}
}

func locationZero() cloudkit.LocationZero {
	return cloudkit.LocationZero{Value: cloudkit.LocationZeroValue0, Type: cloudkit.LocationZeroTypeLocationIntegerType}
}

func locationEncryptedDouble(value float64) cloudkit.LocationEncryptedDouble {
	return cloudkit.LocationEncryptedDouble{Value: locationNumber(value),
		IsEncrypted: cloudkit.LocationEncryptedDoubleEncryptedTrue,
		Type:        cloudkit.LocationEncryptedDoubleTypeLocationEncryptedDoubleType}
}

func locationAlarmRecord(alarm ReminderAlarm, now time.Time) cloudkit.LocationAlarmRecord {
	seconds := float64(now.Unix()) + float64(now.Nanosecond())/float64(time.Second)
	nonce := float64(cloudkit.LocationNonceOffsetValue) + (seconds - float64(cloudkit.ReminderAppleEpochUnixSeconds))

	return cloudkit.LocationAlarmRecord{RecordName: alarm.ID, RecordType: cloudkit.LocationAlarmRecordType,
		PluginFields: map[string]any{},
		Parent:       cloudkit.CKWriteParent{RecordName: alarm.ReminderID, AdditionalProperties: nil},
		Fields: cloudkit.LocationAlarmFields{AlarmUID: locationString(alarm.AlarmUID),
			Deleted: locationZero(), Imported: locationZero(),
			Reminder: locationReference(alarm.ReminderID), TriggerID: locationString(alarm.TriggerID),
			DueDateResolutionTokenAsNonce: cloudkit.LocationDouble{Value: locationNumber(nonce),
				Type: cloudkit.LocationDoubleTypeLocationDoubleType}}}
}

func locationTriggerRecord(trigger ReminderLocationTrigger) cloudkit.LocationTriggerRecord {
	return cloudkit.LocationTriggerRecord{RecordName: trigger.ID, RecordType: cloudkit.LocationTriggerRecordType,
		PluginFields: map[string]any{},
		Parent:       cloudkit.CKWriteParent{RecordName: trigger.AlarmID, AdditionalProperties: nil},
		Fields: cloudkit.LocationTriggerFields{Address: locationEncryptedString(trigger.Address),
			Alarm: locationReference(trigger.AlarmID), Deleted: locationZero(),
			Latitude: locationEncryptedDouble(trigger.Latitude), LocationUID: locationString(trigger.LocationUID),
			Longitude: locationEncryptedDouble(trigger.Longitude),
			Proximity: cloudkit.LocationProximity{Value: cloudkit.LocationProximityValue(trigger.Proximity),
				Type: cloudkit.LocationProximityTypeLocationIntegerType},
			Radius: cloudkit.LocationRadius{Value: locationNumber(trigger.Radius),
				Type: cloudkit.LocationRadiusTypeLocationDoubleType},
			ReferenceFrameString: cloudkit.LocationFrame{Value: cloudkit.LocationFrameValue0,
				IsEncrypted: cloudkit.LocationFrameEncryptedTrue, Type: cloudkit.LocationFrameTypeLocationEncryptedStringType},
			Title: locationEncryptedString(trigger.Title),
			Type:  cloudkit.LocationKind{Value: cloudkit.LocationKindValue0, Type: cloudkit.LocationKindTypeLocationStringType}}}
}

func locationObjects(request AddReminderLocationTriggerRequest, ids []string) (ReminderAlarm, ReminderLocationTrigger) {
	alarm := ReminderAlarm{ID: protocol.RemindersAlarmIDPrefixValue + ids[0],
		ReminderID: reminderRelatedRecordName(request.Reminder.ID, protocol.RemindersReminderIDPrefixValue),
		AlarmUID:   ids[0], TriggerID: ids[1], RecordChangeTag: nil}
	alarm.RecordChangeTag.SetNull()
	trigger := ReminderLocationTrigger{ID: protocol.RemindersAlarmTriggerIDPrefixValue + ids[1],
		AlarmID: alarm.ID, LocationUID: ids[2], Title: request.Title, Address: request.Address,
		Latitude: request.Latitude, Longitude: request.Longitude, Radius: float64(cloudkit.LocationDefaultRadiusValue),
		Proximity: ReminderArriving, RecordChangeTag: nil}
	trigger.RecordChangeTag.SetNull()

	if request.Radius != nil {
		trigger.Radius = *request.Radius
	}

	if request.Proximity != nil {
		trigger.Proximity = ReminderLocationTriggerProximity(*request.Proximity)
	}

	return alarm, trigger
}

func locationReference(name string) cloudkit.LocationReference {
	return cloudkit.LocationReference{Type: cloudkit.LocationReferenceTypeLocationReferenceType,
		Value: cloudkit.ReminderWriteReferenceValue{RecordName: name,
			Action: cloudkit.ReminderWriteReferenceValueActionVALIDATE}}
}
