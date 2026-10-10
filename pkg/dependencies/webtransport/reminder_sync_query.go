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
	input := new(cloudkit.ReminderCurrentSyncQueryRequest)
	input.Query.RecordType = cloudkit.ReminderList
	input.ZoneID = reminderRequestZone()
	input.ResultsLimit = cloudkit.ReminderSyncQueryLimitOne

	body, err := referenceJSON(input)
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
