package icloud

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const deleteReminderHashtagOperation = "DeleteReminderHashtag"

var errReminderChildParent = errors.New("child is linked to a different reminder")

// DeleteReminderHashtag soft-deletes a hashtag and removes every matching raw or full parent link atomically.
func (sdk *SDK) DeleteReminderHashtag(ctx context.Context,
	request DeleteReminderHashtagRequest,
) (*ReminderHashtagRelationResult, error) {
	auth, err := reminderWriteContext(ctx, request.Auth, deleteReminderHashtagOperation)
	if err != nil {
		return nil, err
	}

	reminder := copyReminder(request.Reminder)

	child := copyReminderHashtag(request.Hashtag)

	if child.ReminderID != "" && reminderRecordName(child.ReminderID) != reminderRecordName(reminder.ID) {
		return nil, newClientError(deleteReminderHashtagOperation, Configuration, 0, nil, nil, errReminderChildParent)
	}

	name := reminderRelatedRecordName(child.ID, protocol.RemindersHashtagIDPrefixValue)
	ids := reminderUnlinkedIDs(reminder.HashtagIDs, protocol.RemindersHashtagIDPrefixValue,
		strings.TrimPrefix(name, protocol.RemindersHashtagIDPrefixValue))

	input, err := sdk.reminderHashtagDeletionRequest(reminder, child, ids, name)
	if err != nil {
		return nil, newClientError(deleteReminderHashtagOperation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.DeleteReminderHashtag(ctx, auth, input)
	if err != nil {
		return nil, adaptFailure(deleteReminderHashtagOperation, err)
	}

	tags, err := reminderAcknowledgements(deleteReminderHashtagOperation, response)
	if err != nil {
		return nil, err
	}

	reminder.HashtagIDs = ids
	reminder.RecordChangeTag = reminderAcknowledgedTag(reminder.RecordChangeTag, reminderRecordName(reminder.ID), tags)
	child.RecordChangeTag = reminderAcknowledgedTag(child.RecordChangeTag, name, tags)

	return &ReminderHashtagRelationResult{Reminder: reminder, Hashtag: child,
		Responses: []ResponseMetadata{publicMetadata(response.Metadata)}}, nil
}

func (sdk *SDK) reminderHashtagDeletionRequest(reminder Reminder, child ReminderHashtag,
	ids []string, name string,
) (cloudkit.ReminderHashtagDeletionRequest, error) {
	parent, err := sdk.reminderHashtagParent(reminder, ids)
	if err != nil {
		return cloudkit.ReminderHashtagDeletionRequest{}, err
	}

	fields := cloudkit.ReminderHashtagDeletionFields{Deleted: reminderWriteInteger(1), Reminder: nil}

	if child.ReminderID != "" {
		reference := reminderWriteReference(reminderRecordName(child.ReminderID))
		fields.Reminder = &reference
	}

	operations := make([]cloudkit.ReminderHashtagDeletionRequest_Operations_Item, reminderLinkedOperationCount)

	err = operations[0].FromReminderHashtagParentOperation(parent)
	if err != nil {
		return cloudkit.ReminderHashtagDeletionRequest{}, fmt.Errorf("encode hashtag parent unlink: %w", err)
	}

	record := cloudkit.ReminderHashtagDeletionRecord{RecordName: name,
		RecordType: cloudkit.ReminderHashtagDeletionRecordTypeHashtag, Fields: fields, PluginFields: map[string]any{},
		RecordChangeTag: reminderRevisionPointer(child.RecordChangeTag)}

	err = operations[1].FromReminderHashtagDeletionOperation(cloudkit.ReminderHashtagDeletionOperation{
		OperationType: cloudkit.ReminderHashtagDeletionOperationType, Record: record})
	if err != nil {
		return cloudkit.ReminderHashtagDeletionRequest{}, fmt.Errorf("encode hashtag soft deletion: %w", err)
	}

	return cloudkit.ReminderHashtagDeletionRequest{Operations: operations, ZoneID: reminderWriteZone(),
		Atomic: cloudkit.ReminderHashtagDeletionRequestAtomicEnabled}, nil
}
