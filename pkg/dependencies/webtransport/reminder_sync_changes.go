package webtransport

import (
	"bytes"
	"context"
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/remindersapi"
)

// ReminderSyncChanges reads one token-discovery page without requesting records.
func (client *Client) ReminderSyncChanges(ctx context.Context, auth RequestContext,
	syncToken *string,
) (*ReminderChangesResponse, error) {
	body, err := reminderSyncChangesBody(syncToken)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	params := new(remindersapi.RemindersZoneChangesParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = remindersapi.RemindersZoneChangesParamsRemapEnumsTrue
	params.GetCurrentSyncToken = remindersapi.RemindersZoneChangesParamsGetCurrentSyncTokenTrue

	request, err := remindersapi.NewRemindersZoneChangesRequestWithBody(auth.Origin, params,
		jsonMedia(), bytes.NewReader(body))
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

	return &ReminderChangesResponse{Data: data, Metadata: response}, nil
}

func reminderSyncChangesBody(syncToken *string) ([]byte, error) {
	zone, err := reminderSyncZoneJSON()
	if err != nil {
		return nil, err
	}

	body := fmt.Sprintf("{%q: [{%q: %s, %q: [], %q: []", protocol.RemindersCKZoneChangesRequestZones,
		protocol.RemindersCKZoneChangesZoneReqZoneID, zone, protocol.RemindersCKZoneChangesZoneReqDesiredKeys,
		protocol.RemindersCKZoneChangesZoneReqDesiredRecordTypes)

	if syncToken != nil {
		token, tokenErr := referenceJSON(syncToken)
		if tokenErr != nil {
			return nil, tokenErr
		}

		body += fmt.Sprintf(", %q: %s", protocol.RemindersCKZoneChangesZoneReqSyncToken, token)
	}

	return []byte(body + fmt.Sprintf(", %q: %t}]}", protocol.RemindersCKZoneChangesZoneReqReverse, false)), nil
}
