package webtransport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/remindersapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// ReminderQueryResponse owns decoded query data and exact response evidence.
type ReminderQueryResponse struct {
	Data     cloudkit.CKQueryResponse
	Metadata *BytesResponse
}

// ReminderCurrentSyncQuery performs the Source's lightweight current-token query.
// Decode failures retain the response because the caller may fall back to changes.
func (client *Client) ReminderCurrentSyncQuery(ctx context.Context,
	auth RequestContext,
) (*ReminderQueryResponse, error) {
	zone, err := reminderSyncZoneJSON()
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	body := fmt.Sprintf("{%q: {%q: %q}, %q: %s, %q: %d}", protocol.RemindersCKQueryRequestQuery,
		protocol.RemindersCKQueryObjectRecordType, protocol.RemindersReminderSyncQueryRecordTypeValue,
		protocol.RemindersCKQueryRequestZoneID, zone, protocol.RemindersCKQueryRequestResultsLimit,
		cloudkit.ReminderSyncQueryLimitOne)
	params := new(remindersapi.RemindersQueryRecordsParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = remindersapi.RemindersQueryRecordsParamsRemapEnumsTrue
	params.GetCurrentSyncToken = remindersapi.RemindersQueryRecordsParamsGetCurrentSyncTokenTrue

	request, err := remindersapi.NewRemindersQueryRecordsRequestWithBody(auth.Origin, params,
		protocol.RemindersMediaApplicationJson, bytes.NewBufferString(body))
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	response, err := client.readReminderRequest(ctx, auth, request)
	if err != nil {
		return nil, err
	}

	result := &ReminderQueryResponse{Data: cloudkit.CKQueryResponse{Records: nil,
		ContinuationMarker: nil, SyncToken: nil, AdditionalProperties: nil}, Metadata: response}

	result.Data, err = decodeReminderSyncQuery(response.Body)
	if err != nil {
		return result, responseFailure(Decode, err, response)
	}

	return result, nil
}

func decodeReminderSyncQuery(body []byte) (cloudkit.CKQueryResponse, error) {
	var data cloudkit.CKQueryResponse

	fields, err := accountFields(body)
	if err != nil {
		return data, err
	}

	if raw, exists := fields[protocol.RemindersCKQueryResponseRecords]; exists {
		err = validateReminderChangeRecords(raw)
		if err != nil {
			return data, err
		}
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return data, fmt.Errorf("decode reminder sync query: %w", err)
	}

	err = validateReminderSyncRecords(data.Records)
	if err != nil {
		return data, err
	}

	return data, nil
}

func reminderSyncZoneJSON() ([]byte, error) {
	zone := cloudkit.CKZoneID{ZoneName: protocol.RemindersZoneNameValue,
		OwnerRecordName: nil, ZoneType: nil, AdditionalProperties: nil}
	zone.ZoneType.Set(protocol.RemindersZoneTypeValue)

	return referenceJSON(zone)
}
