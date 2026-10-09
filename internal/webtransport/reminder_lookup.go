package webtransport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/internal/remindersapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// ReminderLookupResponse owns decoded records and the exact provider response.
type ReminderLookupResponse struct {
	Data     cloudkit.CKLookupResponse
	Metadata *BytesResponse
}

// LookupReminder reads one record using schema-owned names and Source body order.
func (client *Client) LookupReminder(ctx context.Context, auth RequestContext,
	name string,
) (*ReminderLookupResponse, error) {
	zone := cloudkit.CKZoneID{ZoneName: protocol.RemindersZoneNameValue,
		OwnerRecordName: nil, ZoneType: nil, AdditionalProperties: nil}
	zone.ZoneType.Set(protocol.RemindersZoneTypeValue)

	identity, err := referenceJSON(zone)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	recordName, err := referenceJSON(name)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	body := fmt.Sprintf("{%q: [{%q: %s}], %q: %s}", protocol.RemindersCKLookupRequestRecords,
		protocol.RemindersCKLookupDescriptorRecordName, recordName, protocol.RemindersCKLookupRequestZoneID, identity)
	params := new(remindersapi.RemindersLookupRecordsParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = remindersapi.RemindersLookupRecordsParamsRemapEnumsTrue
	params.GetCurrentSyncToken = remindersapi.RemindersLookupRecordsParamsGetCurrentSyncTokenTrue

	request, err := remindersapi.NewRemindersLookupRecordsRequestWithBody(auth.Origin, params,
		protocol.RemindersMediaApplicationJson, bytes.NewBufferString(body))
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	response, err := client.readReminderRequest(ctx, auth, request)
	if err != nil {
		return nil, err
	}

	fields, err := accountFields(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	err = validateReminderChangeRecords(fields[protocol.RemindersCKLookupResponseRecords])
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	var data cloudkit.CKLookupResponse

	err = json.Unmarshal(response.Body, &data)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &ReminderLookupResponse{Data: data, Metadata: response}, nil
}
