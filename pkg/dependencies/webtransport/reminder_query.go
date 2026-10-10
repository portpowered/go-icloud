package webtransport

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/remindersapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// ReminderCompoundQuery reads one compound reminder page using caller-owned authentication.
func (client *Client) ReminderCompoundQuery(ctx context.Context, auth RequestContext,
	listID string, completed bool, limit int64, continuation *string,
) (*ReminderQueryResponse, error) {
	body, err := reminderCompoundBody(listID, completed, limit, continuation)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	params := new(remindersapi.RemindersQueryRecordsParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = remindersapi.RemindersQueryRecordsParamsRemapEnumsTrue
	params.GetCurrentSyncToken = remindersapi.RemindersQueryRecordsParamsGetCurrentSyncTokenTrue

	request, err := remindersapi.NewRemindersQueryRecordsRequestWithBody(auth.Origin, params,
		jsonMedia(), bytes.NewReader(body))
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	response, err := client.readReminderRequest(ctx, auth, request)
	if err != nil {
		return nil, err
	}

	data, err := decodeReminderSyncQuery(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &ReminderQueryResponse{Data: data, Metadata: response}, nil
}

func reminderCompoundBody(listID string, completed bool, limit int64, continuation *string) ([]byte, error) {
	parent, parentErr := reminderListQueryFilter(listID)
	include, includeErr := reminderIncludeCompletedFilter(completed)
	validate, validateErr := reminderValidateReferenceFilter()

	err := errors.Join(parentErr, includeErr, validateErr)
	if err != nil {
		return nil, err
	}

	input := new(cloudkit.ReminderCompoundQueryRequest)
	input.Query.RecordType = cloudkit.ReminderList
	input.Query.FilterBy = []cloudkit.ReminderCompoundQueryFilter{parent, include, validate}
	input.ZoneID = reminderRequestZone()
	input.ResultsLimit = limit
	input.ContinuationMarker = continuation

	return referenceJSON(input)
}

func reminderListQueryFilter(listID string) (cloudkit.ReminderCompoundQueryFilter, error) {
	filter := new(cloudkit.ReminderListQueryFilter)
	filter.Comparator = cloudkit.ReminderReadComparatorEquals
	filter.FieldName = cloudkit.ReminderListQueryFilterFieldNameList
	filter.FieldValue.Type = cloudkit.ReminderReadReferenceTypeReference
	filter.FieldValue.Value.RecordName = listID
	filter.FieldValue.Value.Action = cloudkit.ReminderReadReferenceActionValidate
	result := new(cloudkit.ReminderCompoundQueryFilter)

	err := result.FromReminderListQueryFilter(*filter)
	if err != nil {
		return *result, fmt.Errorf("encode reminder list filter: %w", err)
	}

	return *result, nil
}

func reminderIncludeCompletedFilter(completed bool) (cloudkit.ReminderCompoundQueryFilter, error) {
	filter := new(cloudkit.ReminderIncludeCompletedFilter)
	filter.Comparator = cloudkit.ReminderReadComparatorEquals
	filter.FieldName = cloudkit.ReminderIncludeCompletedFilterFieldNameIncludeCompleted
	filter.FieldValue.Type = cloudkit.ReminderReadIntegerTypeInt64
	filter.FieldValue.Value = cloudkit.ReminderIncludeCompletedFalse

	if completed {
		filter.FieldValue.Value = cloudkit.ReminderIncludeCompletedTrue
	}

	result := new(cloudkit.ReminderCompoundQueryFilter)

	err := result.FromReminderIncludeCompletedFilter(*filter)
	if err != nil {
		return *result, fmt.Errorf("encode reminder completed filter: %w", err)
	}

	return *result, nil
}

func reminderValidateReferenceFilter() (cloudkit.ReminderCompoundQueryFilter, error) {
	filter := new(cloudkit.ReminderValidateReferenceFilter)
	filter.Comparator = cloudkit.ReminderReadComparatorEquals
	filter.FieldName = cloudkit.ReminderValidateReferenceFilterFieldNameLookupValidatingReference
	filter.FieldValue.Type = cloudkit.ReminderReadIntegerTypeInt64
	filter.FieldValue.Value = cloudkit.ReminderValidateReferenceEnabled
	result := new(cloudkit.ReminderCompoundQueryFilter)

	err := result.FromReminderValidateReferenceFilter(*filter)
	if err != nil {
		return *result, fmt.Errorf("encode reminder reference filter: %w", err)
	}

	return *result, nil
}
