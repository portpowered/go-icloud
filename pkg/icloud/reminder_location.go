package icloud

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const (
	addReminderLocationOperation   = "AddReminderLocationTrigger"
	reminderLocationOperationCount = reminderLinkedOperationCount + 1
)

var errReminderLocationGeometry = errors.New(
	"location radius must be nonnegative and proximity must be arrival or departure")

// AddReminderLocationTrigger atomically links a new alarm and geofence to a reminder.
func (sdk *SDK) AddReminderLocationTrigger(ctx context.Context,
	request AddReminderLocationTriggerRequest,
) (*AddReminderLocationTriggerResult, error) {
	auth, err := reminderWriteContext(ctx, request.Auth, addReminderLocationOperation)
	if err != nil {
		return nil, err
	}

	ids, err := sdk.locationIdentities()
	if err != nil {
		return nil, newClientError(addReminderLocationOperation, Configuration, 0, nil, nil, err)
	}

	now, nonceTime := sdk.clock(), sdk.clock()

	alarm, trigger := locationObjects(request, ids)
	if trigger.Radius < 0 || math.IsNaN(trigger.Radius) || !trigger.Proximity.Valid() {
		return nil, newClientError(addReminderLocationOperation, Configuration, 0, nil, nil, errReminderLocationGeometry)
	}

	reminder := copyReminder(request.Reminder)
	reminder.AlarmIDs = append(reminderRelatedIDs(reminder.AlarmIDs, protocol.RemindersAlarmIDPrefixValue, ""), ids[0])

	input, err := sdk.locationRequest(reminder, alarm, trigger, now, nonceTime)
	if err != nil {
		return nil, newClientError(addReminderLocationOperation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.AddReminderLocationTrigger(ctx, auth, input)
	if err != nil {
		return nil, adaptFailure(addReminderLocationOperation, err)
	}

	tags, err := reminderAcknowledgements(addReminderLocationOperation, response)
	if err != nil {
		return nil, err
	}

	reminder.RecordChangeTag = reminderAcknowledgedTag(reminder.RecordChangeTag, reminderRecordName(reminder.ID), tags)
	alarm.RecordChangeTag = reminderAcknowledgedTag(alarm.RecordChangeTag, alarm.ID, tags)
	trigger.RecordChangeTag = reminderAcknowledgedTag(trigger.RecordChangeTag, trigger.ID, tags)

	return &AddReminderLocationTriggerResult{Reminder: reminder, Alarm: alarm, Trigger: trigger,
		Responses: []ResponseMetadata{publicMetadata(response.Metadata)}}, nil
}

func (sdk *SDK) locationIdentities() ([]string, error) {
	const count = 3

	ids := make([]string, count)
	for index := range ids {
		identity, err := sdk.randomUUID()
		if err != nil {
			return nil, err
		}

		ids[index] = identity
	}

	return ids, nil
}

func (sdk *SDK) locationRequest(reminder Reminder, alarm ReminderAlarm,
	trigger ReminderLocationTrigger, now, nonceTime time.Time,
) (cloudkit.ReminderLocationRequest, error) {
	alarmRecord := locationAlarmRecord(alarm, nonceTime)

	parent, err := sdk.locationParentOperation(reminder, now)
	if err != nil {
		return cloudkit.ReminderLocationRequest{}, fmt.Errorf("encode location operations: %w", err)
	}

	operations := make([]cloudkit.ReminderLocationOperation, reminderLocationOperationCount)

	err = operations[0].FromLocationParentOperation(parent)
	if err != nil {
		return cloudkit.ReminderLocationRequest{}, fmt.Errorf("encode location operations: %w", err)
	}

	err = operations[1].FromLocationAlarmOperation(cloudkit.LocationAlarmOperation{
		OperationType: cloudkit.LocationAlarmOperationType, Record: alarmRecord,
	})
	if err != nil {
		return cloudkit.ReminderLocationRequest{}, fmt.Errorf("encode location operations: %w", err)
	}

	err = operations[2].FromLocationTriggerOperation(cloudkit.LocationTriggerOperation{
		OperationType: cloudkit.LocationTriggerOperationType, Record: locationTriggerRecord(trigger),
	})
	if err != nil {
		return cloudkit.ReminderLocationRequest{}, fmt.Errorf("encode location operations: %w", err)
	}

	return cloudkit.ReminderLocationRequest{Operations: operations, ZoneID: reminderWriteZone(),
		Atomic: cloudkit.ReminderLocationAtomicTrue}, nil
}
