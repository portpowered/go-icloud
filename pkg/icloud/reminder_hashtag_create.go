package icloud

import (
	"context"
	"fmt"
	"time"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const createReminderHashtagOperation = "CreateReminderHashtag"

// CreateReminderHashtag creates a hashtag and returns independently owned parent and child snapshots.
func (sdk *SDK) CreateReminderHashtag(ctx context.Context,
	request CreateReminderHashtagRequest,
) (*ReminderHashtagRelationResult, error) {
	auth, err := reminderWriteContext(ctx, request.Auth, createReminderHashtagOperation)
	if err != nil {
		return nil, err
	}

	reminder := copyReminder(request.Reminder)

	input, identity, err := sdk.reminderHashtagCreationRequest(reminder, request.Name)
	if err != nil {
		return nil, newClientError(createReminderHashtagOperation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.CreateReminderHashtag(ctx, auth, input)
	if err != nil {
		return nil, adaptFailure(createReminderHashtagOperation, err)
	}

	tags, err := reminderAcknowledgements(createReminderHashtagOperation, response)
	if err != nil {
		return nil, err
	}

	ids := reminderRelatedIDs(reminder.HashtagIDs, protocol.RemindersHashtagIDPrefixValue, "")
	ids = append(ids, identity)
	reminder.HashtagIDs = ids
	parent := reminderRecordName(reminder.ID)
	reminder.RecordChangeTag = reminderAcknowledgedTag(reminder.RecordChangeTag, parent, tags)
	child := ReminderHashtag{ID: protocol.RemindersHashtagIDPrefixValue + identity, Name: request.Name,
		ReminderID: parent, Created: nil, RecordChangeTag: nil}
	child.Created.SetNull()
	child.RecordChangeTag = reminderAcknowledgedTag(nil, child.ID, tags)

	return &ReminderHashtagRelationResult{Reminder: reminder, Hashtag: child,
		Responses: []ResponseMetadata{publicMetadata(response.Metadata)}}, nil
}

func (sdk *SDK) reminderHashtagCreationRequest(reminder Reminder, name string,
) (cloudkit.ReminderHashtagCreationRequest, string, error) {
	now := sdk.clock()

	identity, err := sdk.randomUUID()
	if err != nil {
		return cloudkit.ReminderHashtagCreationRequest{}, "", err
	}

	ids := append(reminderRelatedIDs(reminder.HashtagIDs, protocol.RemindersHashtagIDPrefixValue, ""), identity)

	parent, err := sdk.reminderHashtagParent(reminder, ids)
	if err != nil {
		return cloudkit.ReminderHashtagCreationRequest{}, "", err
	}

	operations := make([]cloudkit.ReminderHashtagCreationRequest_Operations_Item, reminderLinkedOperationCount)

	err = operations[0].FromReminderHashtagParentOperation(parent)
	if err != nil {
		return cloudkit.ReminderHashtagCreationRequest{}, "", fmt.Errorf("encode hashtag parent update: %w", err)
	}

	child := reminderHashtagCreationRecord(reminder, identity, name, now)

	err = operations[1].FromReminderHashtagCreationOperation(cloudkit.ReminderHashtagCreationOperation{
		OperationType: cloudkit.ReminderHashtagCreationOperationType, Record: child})
	if err != nil {
		return cloudkit.ReminderHashtagCreationRequest{}, "", fmt.Errorf("encode hashtag creation: %w", err)
	}

	return cloudkit.ReminderHashtagCreationRequest{Operations: operations, ZoneID: reminderWriteZone(),
		Atomic: cloudkit.ReminderHashtagCreationRequestAtomicEnabled}, identity, nil
}

func reminderHashtagCreationRecord(reminder Reminder, identity, name string,
	now time.Time,
) cloudkit.ReminderHashtagCreationRecord {
	parent := reminderRecordName(reminder.ID)

	return cloudkit.ReminderHashtagCreationRecord{RecordName: protocol.RemindersHashtagIDPrefixValue + identity,
		RecordType: cloudkit.ReminderHashtagCreationRecordTypeHashtag, RecordChangeTag: nil, PluginFields: map[string]any{},
		Parent: cloudkit.CKWriteParent{RecordName: parent, AdditionalProperties: nil},
		Fields: cloudkit.ReminderHashtagCreationFields{
			Name: reminderHashtagName(name), Deleted: reminderFixedZero(), Reminder: reminderRequiredReference(parent),
			CreationDate: reminderRequiredTimestamp(now)}}
}
