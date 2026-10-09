package webtransport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	return client.LookupReminders(ctx, auth, []string{name})
}

// LookupReminders preserves every supplied record name in request order.
func (client *Client) LookupReminders(ctx context.Context, auth RequestContext,
	names []string,
) (*ReminderLookupResponse, error) {
	zone := cloudkit.CKZoneID{ZoneName: protocol.RemindersZoneNameValue,
		OwnerRecordName: nil, ZoneType: nil, AdditionalProperties: nil}
	zone.ZoneType.Set(protocol.RemindersZoneTypeValue)

	identity, err := referenceJSON(zone)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	descriptors := make([]string, 0, len(names))

	for _, name := range names {
		recordName, encodeErr := referenceJSON(name)
		if encodeErr != nil {
			return nil, failure(Configuration, encodeErr, nil, nil)
		}

		descriptors = append(descriptors, fmt.Sprintf("{%q: %s}", protocol.RemindersCKLookupDescriptorRecordName, recordName))
	}

	body := fmt.Sprintf("{%q: [%s], %q: %s}", protocol.RemindersCKLookupRequestRecords,
		strings.Join(descriptors, ", "), protocol.RemindersCKLookupRequestZoneID, identity)
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

	data, err := decodeReminderLookup(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &ReminderLookupResponse{Data: data, Metadata: response}, nil
}

func decodeReminderLookup(body []byte) (cloudkit.CKLookupResponse, error) {
	var data cloudkit.CKLookupResponse

	fields, err := accountFields(body)
	if err != nil {
		return data, fmt.Errorf("decode reminder lookup: %w", err)
	}

	err = validateReminderChangeRecords(fields[protocol.RemindersCKLookupResponseRecords])
	if err != nil {
		return data, fmt.Errorf("decode reminder lookup: %w", err)
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return data, fmt.Errorf("decode reminder lookup: %w", err)
	}

	err = validateReminderSyncRecords(&data.Records)
	if err != nil {
		return data, fmt.Errorf("decode reminder lookup: %w", err)
	}

	return data, nil
}
