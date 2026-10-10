package icloud

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const (
	deleteReminderOperation = "DeleteReminder"
)

// DeleteReminder soft-deletes a reminder and returns acknowledged revision updates.
func (sdk *SDK) DeleteReminder(ctx context.Context, request DeleteReminderRequest) (*DeleteReminderResult, error) {
	err := ctx.Err()
	if err != nil {
		return nil, driveContextFailure(deleteReminderOperation, err)
	}

	auth, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(deleteReminderOperation, Configuration, 0, nil, nil, err)
	}

	auth.Origin = request.Auth.RemindersServiceURL

	name := request.ReminderID
	if name != "" && !strings.HasPrefix(name, protocol.RemindersReminderIDPrefixValue) {
		name = protocol.RemindersReminderIDPrefixValue + name
	}

	now := sdk.clock()

	fields, err := sdk.reminderDeletionFields(now)
	if err != nil {
		return nil, newClientError(deleteReminderOperation, Configuration, 0, nil, nil, err)
	}

	now = time.UnixMilli(fields.LastModifiedDate.Value)

	input := reminderDeletionRequest(name, request, fields)

	response, err := sdk.web.DeleteReminder(ctx, auth, input)
	if err != nil {
		return nil, adaptFailure(deleteReminderOperation, err)
	}

	return reminderDeletionResult(request, name, now, response)
}

func reminderDeletionRequest(name string, request DeleteReminderRequest,
	fields cloudkit.ReminderDeletionFields,
) cloudkit.ReminderDeletionRequest {
	zone := cloudkit.CKZoneIDReq{ZoneName: protocol.RemindersZoneNameValue,
		ZoneType: nil, OwnerRecordName: nil, AdditionalProperties: nil}
	zone.ZoneType.Set(protocol.RemindersZoneTypeValue)

	record := cloudkit.ReminderDeletionRecord{RecordName: name,
		RecordType: cloudkit.ReminderDeletionRecordRecordTypeReminder, Fields: fields,
		PluginFields: map[string]any{}, RecordChangeTag: nil}

	if request.RecordChangeTag.IsSpecified() && !request.RecordChangeTag.IsNull() {
		tag := request.RecordChangeTag.GetOrEmpty()
		record.RecordChangeTag = &tag
	}

	return cloudkit.ReminderDeletionRequest{Operations: []cloudkit.ReminderDeletionOperation{{
		OperationType: cloudkit.ReminderDeletionOperationOperationTypeUpdate, Record: record}}, ZoneID: zone}
}

func reminderDeletionResult(request DeleteReminderRequest, name string, now time.Time,
	response *webtransport.ReminderModificationResponse,
) (*DeleteReminderResult, error) {
	result := &DeleteReminderResult{Deleted: true, Modified: reminderMutationInstant(now),
		RecordChangeTag: maps.Clone(request.RecordChangeTag),
		Responses:       []ResponseMetadata{publicMetadata(response.Metadata)}}

	var records []cloudkit.CKModifyResponse_Records_Item
	if response.Data.Records != nil {
		records = *response.Data.Records
	}

	for _, item := range records {
		selected, decodeErr := webtransport.DecodeReminderModificationRecord(item)
		if decodeErr != nil {
			return nil, deletionResponseFailure(InvalidResponse, decodeErr, response)
		}

		if selected.Failure != nil {
			cause := fmt.Errorf("%w: %s", errReminderList, selected.Failure.ServerErrorCode)

			return nil, deletionResponseFailure(Provider, cause, response)
		}

		if selected.Record != nil && selected.Record.RecordName == name &&
			selected.Record.RecordChangeTag.GetOrEmpty() != "" {
			result.RecordChangeTag = selected.Record.RecordChangeTag
		}
	}

	if !result.RecordChangeTag.IsSpecified() {
		result.RecordChangeTag.SetNull()
	}

	return result, nil
}

func deletionResponseFailure(kind ErrorKind, cause error,
	response *webtransport.ReminderModificationResponse,
) *ClientError {
	metadata := response.Metadata
	failure := newClientError(deleteReminderOperation, kind, metadata.Status, metadata.Body,
		responseHeaders(metadata.Headers), cause)
	failure.cookieScopeURL = metadata.CookieScopeURL

	return failure
}

func (sdk *SDK) reminderDeletionFields(now time.Time) (cloudkit.ReminderDeletionFields, error) {
	deleted, err := sdk.reminderResolutionToken(now)
	if err != nil {
		return cloudkit.ReminderDeletionFields{}, err
	}

	modified, err := sdk.reminderResolutionToken(now)
	if err != nil {
		return cloudkit.ReminderDeletionFields{}, err
	}

	tokens := cloudkit.ReminderDeletionResolutionMap{Map: cloudkit.ReminderDeletionTokens{
		Deleted: deleted, LastModifiedDate: modified}}

	encoded, err := json.Marshal(tokens)
	if err != nil {
		return cloudkit.ReminderDeletionFields{}, fmt.Errorf("encode deletion resolution tokens: %w", err)
	}

	fields := cloudkit.ReminderDeletionFields{
		Deleted: reminderFixedOne(),
		ResolutionTokenMap: cloudkit.ReminderWriteString{Type: cloudkit.ReminderWriteStringTypeSTRING,
			Value: string(encoded)},
		LastModifiedDate: cloudkit.ReminderWriteTimestamp{Type: cloudkit.ReminderWriteTimestampTypeTIMESTAMP,
			Value: reminderTimestamp(sdk.clock()).Value.GetOrEmpty()}}

	return fields, nil
}

func (sdk *SDK) reminderResolutionToken(now time.Time) (cloudkit.ReminderResolutionToken, error) {
	identity, err := sdk.randomUUID()
	if err != nil {
		return cloudkit.ReminderResolutionToken{}, fmt.Errorf("generate reminder resolution identity: %w", err)
	}

	seconds := float64(now.Unix()) + float64(now.Nanosecond())/float64(time.Second) -
		float64(cloudkit.ReminderAppleEpochUnixSeconds)

	stamp := strconv.FormatFloat(seconds, 'f', -1, 64)
	if !strings.Contains(stamp, ".") {
		stamp += ".0"
	}

	return cloudkit.ReminderResolutionToken{Counter: cloudkit.ReminderResolutionTokenCounterN1,
		ModificationTime: json.Number(stamp), ReplicaID: identity}, nil
}
