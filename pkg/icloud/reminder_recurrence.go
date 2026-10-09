package icloud

import (
	"context"
	"errors"
	"maps"
	"slices"

	"github.com/portpowered/go-icloud/internal/protocol"
)

const (
	createReminderRecurrenceOperation = "CreateReminderRecurrenceRule"
	updateReminderRecurrenceOperation = "UpdateReminderRecurrenceRule"
	deleteReminderRecurrenceOperation = "DeleteReminderRecurrenceRule"
	reminderWeekdayMaximum            = 6
)

var (
	errReminderRecurrence         = errors.New("invalid recurrence settings")
	errReminderRecurrenceMutation = errors.New("no recurrence fields supplied")
	errReminderRelationParent     = errors.New("child belongs to another reminder")
)

// CreateReminderRecurrenceRule atomically creates and links a validated recurrence rule.
func (sdk *SDK) CreateReminderRecurrenceRule(ctx context.Context,
	request CreateReminderRecurrenceRuleRequest,
) (*ReminderRecurrenceRuleRelationResult, error) {
	auth, err := reminderWriteContext(ctx, request.Auth, createReminderRecurrenceOperation)
	if err != nil {
		return nil, err
	}

	rule := ReminderRecurrenceRule{ID: "", ReminderID: reminderRecordName(request.Reminder.ID),
		Frequency: ReminderDaily, Interval: 1, OccurrenceCount: 0, FirstDayOfWeek: 0, RecordChangeTag: nil}
	options := UpdateReminderRecurrenceRuleRequest{Auth: request.Auth, RecurrenceRule: rule,
		Frequency: request.Frequency, Interval: request.Interval, OccurrenceCount: request.OccurrenceCount,
		FirstDayOfWeek: request.FirstDayOfWeek}

	rule, err = validatedReminderRecurrence(options)
	if err != nil {
		return nil, newClientError(createReminderRecurrenceOperation, Configuration, 0, nil, nil, err)
	}

	identity, err := sdk.randomUUID()
	if err != nil {
		return nil, newClientError(createReminderRecurrenceOperation, Configuration, 0, nil, nil, err)
	}

	rule.ID = protocol.RemindersRecurrenceRuleIDPrefixValue + identity
	reminder := copyReminder(request.Reminder)
	reminder.RecurrenceRuleIDs = append(reminderRelatedIDs(reminder.RecurrenceRuleIDs,
		protocol.RemindersRecurrenceRuleIDPrefixValue, ""), identity)

	input, err := sdk.reminderRecurrenceCreationRequest(reminder, rule)
	if err != nil {
		return nil, newClientError(createReminderRecurrenceOperation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.CreateReminderRecurrence(ctx, auth, input)
	if err != nil {
		return nil, adaptFailure(createReminderRecurrenceOperation, err)
	}

	tags, err := reminderAcknowledgements(createReminderRecurrenceOperation, response)
	if err != nil {
		return nil, err
	}

	reminder.RecordChangeTag = reminderAcknowledgedTag(reminder.RecordChangeTag, reminderRecordName(reminder.ID), tags)
	rule.RecordChangeTag = reminderAcknowledgedTag(rule.RecordChangeTag, rule.ID, tags)

	return &ReminderRecurrenceRuleRelationResult{Reminder: reminder, RecurrenceRule: rule,
		Responses: []ResponseMetadata{publicMetadata(response.Metadata)}}, nil
}

// UpdateReminderRecurrenceRule validates and writes only supplied recurrence settings.
func (sdk *SDK) UpdateReminderRecurrenceRule(ctx context.Context,
	request UpdateReminderRecurrenceRuleRequest,
) (*ReminderRecurrenceRuleMutationResult, error) {
	auth, err := reminderWriteContext(ctx, request.Auth, updateReminderRecurrenceOperation)
	if err != nil {
		return nil, err
	}

	if request.Frequency == nil && request.Interval == nil &&
		request.OccurrenceCount == nil && request.FirstDayOfWeek == nil {
		return nil, newClientError(updateReminderRecurrenceOperation,
			Configuration, 0, nil, nil, errReminderRecurrenceMutation)
	}

	rule, err := validatedReminderRecurrence(request)
	if err != nil {
		return nil, newClientError(updateReminderRecurrenceOperation, Configuration, 0, nil, nil, err)
	}

	input := reminderRecurrenceUpdateRequest(request)

	response, err := sdk.web.UpdateReminderRecurrence(ctx, auth, input)
	if err != nil {
		return nil, adaptFailure(updateReminderRecurrenceOperation, err)
	}

	tags, err := reminderAcknowledgements(updateReminderRecurrenceOperation, response)
	if err != nil {
		return nil, err
	}

	name := reminderRelatedRecordName(rule.ID, protocol.RemindersRecurrenceRuleIDPrefixValue)
	rule.RecordChangeTag = reminderAcknowledgedTag(rule.RecordChangeTag, name, tags)

	return &ReminderRecurrenceRuleMutationResult{RecurrenceRule: rule,
		Responses: []ResponseMetadata{publicMetadata(response.Metadata)}}, nil
}

// DeleteReminderRecurrenceRule atomically unlinks all matching IDs and soft-deletes the rule.
func (sdk *SDK) DeleteReminderRecurrenceRule(ctx context.Context,
	request DeleteReminderRecurrenceRuleRequest,
) (*ReminderRecurrenceRuleRelationResult, error) {
	auth, err := reminderWriteContext(ctx, request.Auth, deleteReminderRecurrenceOperation)
	if err != nil {
		return nil, err
	}

	reminder := copyReminder(request.Reminder)
	rule := request.RecurrenceRule

	rule.RecordChangeTag = maps.Clone(rule.RecordChangeTag)

	if rule.ReminderID != "" && reminderRecordName(rule.ReminderID) != reminderRecordName(reminder.ID) {
		return nil, newClientError(deleteReminderRecurrenceOperation, Configuration, 0, nil, nil, errReminderRelationParent)
	}

	name := reminderRelatedRecordName(rule.ID, protocol.RemindersRecurrenceRuleIDPrefixValue)
	raw := reminderRelatedIDs([]string{name}, protocol.RemindersRecurrenceRuleIDPrefixValue, "")[0]
	reminder.RecurrenceRuleIDs = reminderUnlinkedRecurrenceIDs(reminder.RecurrenceRuleIDs, raw)

	input, err := sdk.reminderRecurrenceDeletionRequest(reminder, rule)
	if err != nil {
		return nil, newClientError(deleteReminderRecurrenceOperation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.DeleteReminderRecurrence(ctx, auth, input)
	if err != nil {
		return nil, adaptFailure(deleteReminderRecurrenceOperation, err)
	}

	tags, err := reminderAcknowledgements(deleteReminderRecurrenceOperation, response)
	if err != nil {
		return nil, err
	}

	reminder.RecordChangeTag = reminderAcknowledgedTag(reminder.RecordChangeTag, reminderRecordName(reminder.ID), tags)
	rule.RecordChangeTag = reminderAcknowledgedTag(rule.RecordChangeTag, name, tags)

	return &ReminderRecurrenceRuleRelationResult{Reminder: reminder, RecurrenceRule: rule,
		Responses: []ResponseMetadata{publicMetadata(response.Metadata)}}, nil
}

func validatedReminderRecurrence(request UpdateReminderRecurrenceRuleRequest) (ReminderRecurrenceRule, error) {
	rule := request.RecurrenceRule

	rule.RecordChangeTag = maps.Clone(rule.RecordChangeTag)

	if request.Frequency != nil {
		rule.Frequency = *request.Frequency
	}

	if request.Interval != nil {
		rule.Interval = *request.Interval
	}

	if request.OccurrenceCount != nil {
		rule.OccurrenceCount = *request.OccurrenceCount
	}

	if request.FirstDayOfWeek != nil {
		rule.FirstDayOfWeek = *request.FirstDayOfWeek
	}

	if !validReminderRecurrence(rule) {
		return ReminderRecurrenceRule{}, errReminderRecurrence
	}

	return rule, nil
}

func validReminderRecurrence(rule ReminderRecurrenceRule) bool {
	return rule.Frequency >= ReminderDaily && rule.Frequency <= ReminderYearly && rule.Interval >= 1 &&
		rule.OccurrenceCount >= 0 && rule.FirstDayOfWeek >= 0 && rule.FirstDayOfWeek <= reminderWeekdayMaximum
}

func reminderUnlinkedRecurrenceIDs(input []string, removed string) []string {
	ids := reminderRelatedIDs(input, protocol.RemindersRecurrenceRuleIDPrefixValue, "")

	return slices.DeleteFunc(ids, func(name string) bool { return name == removed })
}
