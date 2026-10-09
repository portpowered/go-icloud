package webtransport

import (
	"bytes"
	"context"
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/internal/remindersapi"
)

// ReminderEventChanges reads one ordered reminder change page from a caller cursor.
func (client *Client) ReminderEventChanges(ctx context.Context, auth RequestContext,
	syncToken *string,
) (*ReminderChangesResponse, error) {
	body, err := reminderEventChangesBody(syncToken)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	params := new(remindersapi.RemindersZoneChangesParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = remindersapi.RemindersZoneChangesParamsRemapEnumsTrue
	params.GetCurrentSyncToken = remindersapi.RemindersZoneChangesParamsGetCurrentSyncTokenTrue

	request, err := remindersapi.NewRemindersZoneChangesRequestWithBody(auth.Origin, params,
		protocol.RemindersMediaApplicationJson, bytes.NewReader(body))
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	response, err := client.readReminderRequest(ctx, auth, request)
	if err != nil {
		return nil, err
	}

	data, err := decodeReminderChanges(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	if data.Zones != nil {
		for _, zone := range *data.Zones {
			err = validateReminderSyncRecords(zone.Records)
			if err != nil {
				return nil, responseFailure(Decode, err, response)
			}
		}
	}

	return &ReminderChangesResponse{Data: data, Metadata: response}, nil
}

func reminderEventChangesBody(syncToken *string) ([]byte, error) {
	zone, err := reminderSyncZoneJSON()
	if err != nil {
		return nil, err
	}

	types, err := referenceJSON([]string{protocol.RemindersReminderRecordTypeValue})
	if err != nil {
		return nil, err
	}

	body := fmt.Sprintf("{%q: [{%q: %s, %q: %s", protocol.RemindersCKZoneChangesRequestZones,
		protocol.RemindersCKZoneChangesZoneReqZoneID, zone, protocol.RemindersCKZoneChangesZoneReqDesiredRecordTypes, types)

	if syncToken != nil {
		token, tokenErr := referenceJSON(syncToken)
		if tokenErr != nil {
			return nil, tokenErr
		}

		body += fmt.Sprintf(", %q: %s", protocol.RemindersCKZoneChangesZoneReqSyncToken, token)
	}

	return []byte(body + fmt.Sprintf(", %q: %t}]}", protocol.RemindersCKZoneChangesZoneReqReverse, false)), nil
}
