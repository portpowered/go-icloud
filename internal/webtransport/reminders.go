package webtransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/portpowered/go-icloud/internal/accountapi"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/internal/remindersapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

var errReminderZonesShape = errors.New("reminder response has invalid zone identities")

// ReminderZonesResponse owns decoded zone records and response metadata.
type ReminderZonesResponse struct {
	Data     cloudkit.CKZoneListResponse
	Metadata *BytesResponse
}

// ListReminderZones discovers fresh zones using operation-local credentials.
func (client *Client) ListReminderZones(ctx context.Context, auth RequestContext) (*ReminderZonesResponse, error) {
	params := new(remindersapi.RemindersListZonesParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = remindersapi.RemindersListZonesParamsRemapEnumsTrue
	params.GetCurrentSyncToken = remindersapi.RemindersListZonesParamsGetCurrentSyncTokenTrue

	request, err := remindersapi.NewRemindersListZonesRequestWithBody(auth.Origin, params,
		protocol.RemindersMediaApplicationJson, strings.NewReader("{}"))
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	err = validateOrigin(auth.Origin)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	request = request.WithContext(ctx)
	request.Header = auth.Headers.Clone()
	request.Header.Set(protocol.HTTPContentTypeName, protocol.RemindersMediaApplicationJson)

	if request.Header.Get(protocol.AcceptName) == "" {
		request.Header.Set(protocol.AcceptName, string(accountapi.AcceptAsterisk))
	}

	request.URL.RawQuery = queryPart(protocol.RemindersRemapEnumsName, string(params.RemapEnums)) + "&" +
		queryPart(protocol.RemindersGetCurrentSyncTokenName, string(params.GetCurrentSyncToken)) + "&" +
		orderedAccountQuery(auth.Params)

	response, err := client.readPrepared(request, successfulContent, auth.Cookies)
	if err != nil {
		return nil, err
	}

	data, err := decodeReminderZones(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &ReminderZonesResponse{Data: data, Metadata: response}, nil
}

func decodeReminderZones(body []byte) (cloudkit.CKZoneListResponse, error) {
	var data cloudkit.CKZoneListResponse

	fields, err := accountFields(body)
	if err != nil {
		return data, err
	}

	raw, exists := fields[protocol.RemindersCKZoneListResponseZones]
	if exists {
		var zones []map[string]json.RawMessage

		err = json.Unmarshal(raw, &zones)
		if err != nil || zones == nil {
			return data, errReminderZonesShape
		}

		for _, zone := range zones {
			identity, identityErr := accountFields(zone[protocol.RemindersCKZoneListZoneZoneID])
			if identityErr != nil || len(identity[protocol.RemindersCKZoneIDZoneName]) == 0 ||
				string(identity[protocol.RemindersCKZoneIDZoneName]) == jsonNullValue {
				return data, errReminderZonesShape
			}
		}
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return data, fmt.Errorf("decode reminder zones: %w", err)
	}

	return data, nil
}
