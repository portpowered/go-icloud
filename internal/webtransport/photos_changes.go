package webtransport

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/portpowered/go-icloud/internal/accountapi"
	"github.com/portpowered/go-icloud/internal/photosapi"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// PhotosChangesResponse retains a validated page and exact response evidence.
type PhotosChangesResponse struct {
	Data     cloudkit.CKZoneChangesResponse
	Metadata *BytesResponse
}

// PhotosChanges reads changes from the selected database and zone.
func (client *Client) PhotosChanges(
	ctx context.Context,
	auth RequestContext,
	since *string,
) (*PhotosChangesResponse, error) {
	zone := cloudkit.CKZoneIDReq{
		ZoneName:             protocol.PhotosPhotoPrimaryZoneNameValue,
		ZoneType:             nil,
		OwnerRecordName:      nil,
		AdditionalProperties: nil,
	}
	zone.ZoneType.Set(protocol.PhotosPhotoPrimaryZoneTypeValue)

	if auth.PhotoZone != nil {
		zone = *auth.PhotoZone
	}

	identity, err := referenceJSONFields(
		zone,
		[]string{
			protocol.PhotosCKZoneIDReqZoneName,
			protocol.PhotosCKZoneIDReqZoneType,
			protocol.PhotosCKZoneIDReqOwnerRecordName,
		},
	)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	body := fmt.Sprintf(
		"{%q: [{%q: %s",
		protocol.PhotosCKZoneChangesRequestZones,
		protocol.PhotosCKZoneChangesZoneReqZoneID,
		identity,
	)

	if since != nil {
		token, tokenErr := referenceJSON(*since)
		if tokenErr != nil {
			return nil, failure(Configuration, tokenErr, nil, nil)
		}

		body += fmt.Sprintf(", %q: %s", protocol.PhotosCKZoneChangesZoneReqSyncToken, token)
	}

	body += fmt.Sprintf(", %q: false}]}", protocol.PhotosCKZoneChangesZoneReqReverse)

	request, err := photosChangesRequest(auth, body)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	response, err := client.readPhotosRequest(ctx, auth, request)
	if err != nil {
		return nil, err
	}

	data, err := decodeReminderChanges(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	err = validatePhotosChangePages(data)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &PhotosChangesResponse{Data: data, Metadata: response}, nil
}

func photosChangesRequest(auth RequestContext, body string) (*http.Request, error) {
	if auth.PhotoShared {
		params := new(photosapi.PhotosSharedZoneChangesParams)
		params.ClientId = auth.Params.ClientId
		params.Dsid = auth.Params.Dsid
		params.RemapEnums = photosapi.PhotosSharedZoneChangesParamsRemapEnumsTrue
		params.GetCurrentSyncToken = photosapi.PhotosSharedZoneChangesParamsGetCurrentSyncTokenTrue

		request, err := photosapi.NewPhotosSharedZoneChangesRequestWithBody(
			auth.Origin,
			params,
			protocol.PhotosMediaApplicationJson,
			bytes.NewBufferString(body),
		)
		if err != nil {
			return nil, fmt.Errorf("construct shared photo changes: %w", err)
		}

		return request, nil
	}

	params := new(photosapi.PhotosZoneChangesParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = photosapi.PhotosZoneChangesParamsRemapEnumsTrue
	params.GetCurrentSyncToken = photosapi.PhotosZoneChangesParamsGetCurrentSyncTokenTrue

	request, err := photosapi.NewPhotosZoneChangesRequestWithBody(
		auth.Origin,
		params,
		protocol.PhotosMediaApplicationJson,
		bytes.NewBufferString(body),
	)
	if err != nil {
		return nil, fmt.Errorf("construct photo changes: %w", err)
	}

	return request, nil
}

func (client *Client) readPhotosRequest(
	ctx context.Context,
	auth RequestContext,
	request *http.Request,
) (*BytesResponse, error) {
	err := validateOrigin(auth.Origin)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	request = request.WithContext(ctx)
	request.Header = auth.Headers.Clone()
	request.Header.Set(protocol.HTTPContentTypeName, protocol.PhotosMediaApplicationJson)

	if request.Header.Get(protocol.AcceptName) == "" {
		request.Header.Set(protocol.AcceptName, string(accountapi.AcceptAsterisk))
	}

	request.URL.RawQuery = orderedAccountQuery(
		auth.Params,
	) + "&" + queryPart(
		protocol.PhotosRemapEnumsName,
		string(photosapi.PhotosZoneChangesParamsRemapEnumsTrue),
	) + "&" + queryPart(
		protocol.PhotosGetCurrentSyncTokenName,
		string(photosapi.PhotosZoneChangesParamsGetCurrentSyncTokenTrue),
	)

	return client.readPrepared(request, successfulContent, auth.Cookies)
}

func validatePhotosChangePages(data cloudkit.CKZoneChangesResponse) error {
	var err error
	if data.Zones != nil {
		for _, page := range *data.Zones {
			err = validateReminderSyncRecords(page.Records)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
