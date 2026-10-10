package webtransport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/remindersapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// ReminderChangesResponse owns a decoded page and its exact HTTP evidence.
type ReminderChangesResponse struct {
	Data     cloudkit.CKZoneChangesResponse
	Metadata *BytesResponse
}

// ReminderListChanges reads one list page using the reference's ordered request body.
func (client *Client) ReminderListChanges(ctx context.Context, auth RequestContext,
	syncToken *string,
) (*ReminderChangesResponse, error) {
	body, err := reminderListChangesBody(syncToken)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	params := &remindersapi.RemindersZoneChangesParams{ClientId: auth.Params.ClientId, Dsid: auth.Params.Dsid,
		RemapEnums:          remindersapi.RemindersZoneChangesParamsRemapEnumsTrue,
		GetCurrentSyncToken: remindersapi.RemindersZoneChangesParamsGetCurrentSyncTokenTrue,
		ClientBuildNumber:   nil, ClientMasteringNumber: nil, Accept: nil, ContentType: nil, Cookie: nil,
		Origin: nil, Referer: nil, UserAgent: nil, AcceptEncoding: nil, Connection: nil}

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

func decodeReminderChanges(body []byte) (cloudkit.CKZoneChangesResponse, error) {
	var data cloudkit.CKZoneChangesResponse

	fields, err := accountFields(body)
	if err != nil {
		return data, err
	}

	if raw, exists := fields[protocol.RemindersCKZoneChangesResponseZones]; exists {
		var zones []json.RawMessage

		err = json.Unmarshal(raw, &zones)
		if err != nil || zones == nil {
			return data, errReminderZonesShape
		}

		for _, zone := range zones {
			err = validateReminderChangeZone(zone)
			if err != nil {
				return data, err
			}
		}
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return data, fmt.Errorf("decode reminder changes: %w", err)
	}

	return data, nil
}

func validateReminderChangeZone(raw json.RawMessage) error {
	fields, err := accountFields(raw)
	if err != nil {
		return err
	}

	identity, err := accountFields(fields[protocol.RemindersCKZoneChangesZoneZoneID])
	if err != nil {
		return err
	}

	if !reminderRequiredValue(identity, protocol.RemindersCKZoneIDZoneName) ||
		!reminderRequiredValue(fields, protocol.RemindersCKZoneChangesZoneSyncToken) {
		return errReminderZonesShape
	}

	if rawRecords, exists := fields[protocol.RemindersCKZoneChangesZoneRecords]; exists {
		return validateReminderChangeRecords(rawRecords)
	}

	return nil
}

func validateReminderChangeRecords(raw json.RawMessage) error {
	var records []json.RawMessage

	err := json.Unmarshal(raw, &records)
	if err != nil || records == nil {
		return errReminderZonesShape
	}

	for _, record := range records {
		fields, fieldsErr := accountFields(record)
		if fieldsErr != nil {
			return fieldsErr
		}

		if reminderRequiredValue(fields, protocol.RemindersCKErrorItemServerErrorCode) {
			continue
		}

		if !reminderRequiredValue(fields, protocol.RemindersCKRecordRecordName) {
			return errReminderZonesShape
		}

		if reminderRequiredValue(fields, protocol.RemindersCKRecordRecordType) {
			continue
		}

		if string(fields[protocol.RemindersCKTombstoneRecordDeleted]) != "true" {
			return errReminderZonesShape
		}
	}

	return nil
}

func reminderRequiredValue(fields map[string]json.RawMessage, name string) bool {
	return len(fields[name]) != 0 && string(fields[name]) != jsonNullValue
}

func reminderListChangesBody(syncToken *string) ([]byte, error) {
	zone := cloudkit.CKZoneID{ZoneName: protocol.RemindersZoneNameValue,
		OwnerRecordName: nil, ZoneType: nil, AdditionalProperties: nil}
	zone.ZoneType.Set(protocol.RemindersZoneTypeValue)

	identity, err := referenceJSON(zone)
	if err != nil {
		return nil, err
	}

	types, err := referenceJSON([]string{protocol.RemindersListRecordTypeValue})
	if err != nil {
		return nil, err
	}
	// Generated extensible models marshal through maps. Preserve Source field insertion
	// order explicitly while leaving names and scalar encodings schema-owned.
	body := fmt.Sprintf("{%q: [{%q: %s, %q: %s", protocol.RemindersCKZoneChangesRequestZones,
		protocol.RemindersCKZoneChangesZoneReqZoneID, identity,
		protocol.RemindersCKZoneChangesZoneReqDesiredRecordTypes, types)

	if syncToken != nil {
		token, tokenErr := referenceJSON(syncToken)
		if tokenErr != nil {
			return nil, tokenErr
		}

		body += fmt.Sprintf(", %q: %s", protocol.RemindersCKZoneChangesZoneReqSyncToken, token)
	}

	return []byte(body + "}]}"), nil
}

func (client *Client) readReminderRequest(ctx context.Context, auth RequestContext,
	request *http.Request,
) (*BytesResponse, error) {
	err := validateOrigin(auth.Origin)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	request = request.WithContext(ctx)
	request.Header = callerHeaders(auth.Headers)
	request.Header.Set(protocol.HTTPContentTypeName, jsonMedia())

	if request.Header.Get(protocol.AcceptName) == "" {
		request.Header.Set(protocol.AcceptName, anyMedia())
	}

	remap := string(remindersapi.RemindersZoneChangesParamsRemapEnumsTrue)
	current := string(remindersapi.RemindersZoneChangesParamsGetCurrentSyncTokenTrue)
	request.URL.RawQuery = queryPart(protocol.RemindersRemapEnumsName, remap) + "&" +
		queryPart(protocol.RemindersGetCurrentSyncTokenName, current) + "&" + orderedAccountQuery(auth.Params)

	return client.readPrepared(request, successfulContent, auth.Cookies)
}

// DownloadReminderMembership fetches only an asset token from a decoded list record.
// The public API never accepts an asset URL. Preserve provider path/query bytes.
func (client *Client) DownloadReminderMembership(ctx context.Context, auth RequestContext,
	asset cloudkit.CKAssetToken,
) (*BytesResponse, error) {
	target, err := contentLocatorTarget(asset.DownloadURL.GetOrEmpty())
	if err != nil || target.Scheme != protocol.DriveHTTPSchemeValue || target.Host == "" ||
		target.User != nil || target.Fragment != "" {
		return nil, failure(Decode, errReminderZonesShape, nil, nil)
	}

	request, err := remindersapi.NewRemindersDownloadAssetRequest(
		target.Scheme+"://"+target.Host, target.EscapedPath(), nil,
	)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	request.URL = target
	request = request.WithContext(ctx)

	request.Header = callerHeaders(auth.Headers)

	if request.Header.Get(protocol.AcceptName) == "" {
		request.Header.Set(protocol.AcceptName, anyMedia())
	}

	return client.readPrepared(request, successfulContent, auth.Cookies)
}
