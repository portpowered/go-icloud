package webtransport

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/accountapi"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/photosapi"
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
	body, err := photosChangesBody(auth, since)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

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

func photosChangesBody(auth RequestContext, since *string) ([]byte, error) {
	zone := cloudkit.PhotosChangeZoneIdentity{ZoneName: protocol.PhotosPhotoPrimaryZoneNameValue,
		ZoneType: nil, OwnerRecordName: nil}
	primaryType := protocol.PhotosPhotoPrimaryZoneTypeValue
	zone.ZoneType = &primaryType
	if auth.PhotoZone != nil {
		zone.ZoneName = auth.PhotoZone.ZoneName
		zone.ZoneType = nil
		value, err := auth.PhotoZone.ZoneType.Get()
		if err == nil {
			zone.ZoneType = &value
		}
		value, err = auth.PhotoZone.OwnerRecordName.Get()
		if err == nil {
			zone.OwnerRecordName = &value
		}
	}
	input := cloudkit.PhotosChangesRequest{Zones: []cloudkit.PhotosChangesZoneRequest{
		{ZoneID: zone, SyncToken: since, Reverse: cloudkit.PhotoChangesForwardValue},
	}}
	return referenceJSON(input)
}

func photosChangesRequest(auth RequestContext, body []byte) (*http.Request, error) {
	if auth.PhotoShared {
		params := photosSharedZoneChangesParams(auth)
		request, err := photosapi.NewPhotosSharedZoneChangesRequestWithBody(
			auth.Origin,
			params,
			protocol.PhotosMediaApplicationJson,
			bytes.NewReader(body),
		)
		if err != nil {
			return nil, fmt.Errorf("construct shared photo changes: %w", err)
		}

		return request, nil
	}

	params := photosZoneChangesParams(auth)
	request, err := photosapi.NewPhotosZoneChangesRequestWithBody(
		auth.Origin,
		params,
		protocol.PhotosMediaApplicationJson,
		bytes.NewReader(body),
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
	return client.readPhotosPrepared(ctx, auth, request,
		string(photosapi.PhotosZoneChangesParamsRemapEnumsTrue),
		string(photosapi.PhotosZoneChangesParamsGetCurrentSyncTokenTrue))
}

func (client *Client) readPhotosContainerRequest(ctx context.Context, auth RequestContext,
	request *http.Request,
) (*BytesResponse, error) {
	return client.readPhotosPrepared(ctx, auth, request,
		string(photosapi.PhotosZoneChangesParamsRemapEnumsTrue),
		string(photosapi.PhotosZoneChangesParamsGetCurrentSyncTokenTrue))
}

func (client *Client) readPhotosPrepared(ctx context.Context, auth RequestContext,
	request *http.Request, remap, current string,
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
		remap,
	) + "&" + queryPart(
		protocol.PhotosGetCurrentSyncTokenName,
		current,
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

func photosSharedZoneChangesParams(auth RequestContext) *photosapi.PhotosSharedZoneChangesParams {
	params := new(photosapi.PhotosSharedZoneChangesParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = photosapi.PhotosSharedZoneChangesParamsRemapEnumsTrue
	params.GetCurrentSyncToken = photosapi.PhotosSharedZoneChangesParamsGetCurrentSyncTokenTrue

	return params
}

func photosZoneChangesParams(auth RequestContext) *photosapi.PhotosZoneChangesParams {
	params := new(photosapi.PhotosZoneChangesParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = photosapi.PhotosZoneChangesParamsRemapEnumsTrue
	params.GetCurrentSyncToken = photosapi.PhotosZoneChangesParamsGetCurrentSyncTokenTrue

	return params
}
