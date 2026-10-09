package icloud

import (
	"context"
	"time"

	"github.com/portpowered/go-icloud/internal/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const (
	createReminderOperation    = "CreateReminder"
	reminderCreationTokenCount = 15
)

// CreateReminder creates a reminder and reads its complete acknowledged snapshot.
func (sdk *SDK) CreateReminder(ctx context.Context, request CreateReminderRequest) (*ReminderMutationResult, error) {
	err := ctx.Err()
	if err != nil {
		return nil, driveContextFailure(createReminderOperation, err)
	}

	auth, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(createReminderOperation, Configuration, 0, nil, nil, err)
	}

	auth.Origin = request.Auth.RemindersServiceURL

	input, err := sdk.reminderCreationRequest(request)
	if err != nil {
		return nil, newClientError(createReminderOperation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.CreateReminder(ctx, auth, input)
	if err != nil {
		return nil, adaptFailure(createReminderOperation, err)
	}

	name := input.Operations[0].Record.RecordName

	_, err = reminderAcknowledgements(createReminderOperation, response)
	if err != nil {
		return nil, err
	}

	return sdk.hydrateCreatedReminder(ctx, auth, name, response.Metadata)
}

func (sdk *SDK) hydrateCreatedReminder(ctx context.Context, auth webtransport.RequestContext,
	name string, prior *webtransport.BytesResponse,
) (*ReminderMutationResult, error) {
	response, err := sdk.web.LookupReminder(ctx, auth, name)
	if err != nil {
		failure := adaptFailure(createReminderOperation, err)
		failure.prior = append([]ResponseMetadata{publicMetadata(prior)}, failure.prior...)

		return nil, failure
	}

	record, kind, err := selectReminderRecord(response.Data, name)
	if err != nil {
		failure := reminderLookupFailure(kind, err, response.Metadata)
		failure.operation = createReminderOperation
		failure.prior = []ResponseMetadata{publicMetadata(prior)}

		return nil, failure
	}

	reminder, err := projectReminder(record)
	if err != nil {
		failure := reminderLookupFailure(InvalidResponse, err, response.Metadata)
		failure.operation = createReminderOperation
		failure.prior = []ResponseMetadata{publicMetadata(prior)}

		return nil, failure
	}

	return &ReminderMutationResult{Reminder: reminder,
		Responses: []ResponseMetadata{publicMetadata(prior), publicMetadata(response.Metadata)}}, nil
}

func (sdk *SDK) reminderCreationRequest(request CreateReminderRequest) (cloudkit.ReminderCreationRequest, error) {
	identity, err := sdk.randomUUID()
	if err != nil {
		return cloudkit.ReminderCreationRequest{}, err
	}

	fields, err := sdk.reminderCreationFields(request)
	if err != nil {
		return cloudkit.ReminderCreationRequest{}, err
	}

	record := cloudkit.ReminderCreationRecord{RecordName: reminderRecordName(identity),
		RecordType: cloudkit.ReminderCreationRecordTypeReminder, Fields: fields, PluginFields: map[string]any{},
		RecordChangeTag: nil, Parent: cloudkit.CKWriteParent{RecordName: request.ListID, AdditionalProperties: nil}}

	return cloudkit.ReminderCreationRequest{Operations: []cloudkit.ReminderCreationOperation{{
		OperationType: cloudkit.ReminderCreationOperationType, Record: record}}, ZoneID: reminderWriteZone()}, nil
}

func (sdk *SDK) reminderCreationFields(request CreateReminderRequest) (cloudkit.ReminderCreationFields, error) {
	title, notes, err := reminderDocuments(request.Title, request.Description)
	if err != nil {
		return cloudkit.ReminderCreationFields{}, err
	}

	tokens, err := sdk.reminderTokens(sdk.clock(), reminderCreationTokenCount)
	if err != nil {
		return cloudkit.ReminderCreationFields{}, err
	}

	resolution, err := reminderTokenString(cloudkit.ReminderCreationTokensMap{Map: cloudkit.ReminderCreationTokens{
		AllDay: tokens[0], TitleDocument: tokens[1], NotesDocument: tokens[2], ParentReminder: tokens[3],
		Priority: tokens[4], IcsDisplayOrder: tokens[5], CreationDate: tokens[6], List: tokens[7], Flagged: tokens[8],
		Completed: tokens[9], CompletionDate: tokens[10], LastModifiedDate: tokens[11], RecurrenceRuleIDs: tokens[12],
		DueDate: tokens[13], TimeZone: tokens[14]}})
	if err != nil {
		return cloudkit.ReminderCreationFields{}, err
	}

	now := sdk.clock()

	fields := cloudkit.ReminderCreationFields{AllDay: reminderBoolean(request.AllDay),
		Completed: reminderBoolean(request.Completed), CompletionDate: cloudkit.ReminderOptionalTimestamp{
			Type: cloudkit.ReminderOptionalTIMESTAMPType, Value: nil}, CreationDate: reminderRequiredTimestamp(now),
		Deleted: reminderFixedZero(), Flagged: reminderBoolean(request.Flagged), Imported: reminderFixedZero(),
		LastModifiedDate: reminderRequiredTimestamp(now), List: reminderRequiredReference(request.ListID),
		NotesDocument: reminderDocument(notes), Priority: reminderWriteInteger(request.Priority),
		ResolutionTokenMap: resolution, TitleDocument: reminderDocument(title),
		DueDate: nil, TimeZone: nil, ParentReminder: nil}
	applyReminderCreationOptions(&fields, request, now)

	return fields, nil
}

func applyReminderCreationOptions(fields *cloudkit.ReminderCreationFields,
	request CreateReminderRequest, now time.Time,
) {
	if request.Completed {
		fields.CompletionDate = reminderTimestamp(now)
	}

	if request.DueDate.IsSpecified() && !request.DueDate.IsNull() {
		stamp := reminderTimestamp(request.DueDate.GetOrEmpty())
		fields.DueDate = &stamp
	}

	if request.TimeZone != "" {
		zone := reminderString(request.TimeZone)
		fields.TimeZone = &zone
	}

	if request.ParentReminderID != "" {
		parent := reminderWriteReference(reminderRecordName(request.ParentReminderID))
		fields.ParentReminder = &parent
	}
}
