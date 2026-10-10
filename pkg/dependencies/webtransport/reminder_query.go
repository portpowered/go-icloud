package webtransport

import (
	"bytes"
	"context"
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
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
		protocol.RemindersMediaApplicationJson, bytes.NewReader(body))
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
	name, err := referenceJSON(listID)
	if err != nil {
		return nil, err
	}

	zone, err := reminderSyncZoneJSON()
	if err != nil {
		return nil, err
	}

	identity := fmt.Sprintf("{%q: %s, %q: %q}", protocol.RemindersCKReferenceRecordName, name,
		protocol.RemindersCKReferenceAction, protocol.RemindersCompoundQueryReferenceActionValue)
	reference := fmt.Sprintf("{%q: %q, %q: %s}", protocol.RemindersCKFVReferenceType,
		cloudkit.CKFVReferenceTypeREFERENCE, protocol.RemindersCKFVReferenceValue, identity)

	include := 0
	if completed {
		include = 1
	}

	includeFilter := reminderCompoundFilter(protocol.RemindersCompoundQueryIncludeCompletedValue,
		reminderCompoundInteger(include))
	validateFilter := reminderCompoundFilter(protocol.RemindersCompoundQueryValidateReferenceValue,
		reminderCompoundInteger(1))
	filters := reminderCompoundFilter(protocol.RemindersReminderFieldListValue, reference) + ", " +
		includeFilter + ", " + validateFilter
	query := fmt.Sprintf("{%q: %q, %q: [%s]}", protocol.RemindersCKQueryObjectRecordType,
		protocol.RemindersReminderSyncQueryRecordTypeValue, protocol.RemindersCKQueryObjectFilterBy, filters)
	body := fmt.Sprintf("{%q: %s, %q: %s, %q: %d", protocol.RemindersCKQueryRequestQuery, query,
		protocol.RemindersCKQueryRequestZoneID, zone, protocol.RemindersCKQueryRequestResultsLimit, limit)

	if continuation != nil {
		marker, markerErr := referenceJSON(*continuation)
		if markerErr != nil {
			return nil, markerErr
		}

		body += fmt.Sprintf(", %q: %s", protocol.RemindersCKQueryRequestContinuationMarker, marker)
	}

	return []byte(body + "}"), nil
}

func reminderCompoundFilter(name, value string) string {
	return fmt.Sprintf("{%q: %q, %q: %q, %q: %s}", protocol.RemindersCKQueryFilterByComparator,
		cloudkit.CKComparatorEQUALS, protocol.RemindersCKQueryFilterByFieldName, name,
		protocol.RemindersCKQueryFilterByFieldValue, value)
}

func reminderCompoundInteger(value int) string {
	return fmt.Sprintf("{%q: %q, %q: %d}", protocol.RemindersCKFVInt64Type,
		cloudkit.CKFVInt64TypeINT64, protocol.RemindersCKFVInt64Value, value)
}
