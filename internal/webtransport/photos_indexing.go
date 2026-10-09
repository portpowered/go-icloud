package webtransport

import (
	"bytes"
	"context"
	"fmt"
	"github.com/portpowered/go-icloud/internal/accountapi"
	"github.com/portpowered/go-icloud/internal/photosapi"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"net/http"
)

// PhotosQueryResponse retains validated CloudKit query records and exact response evidence.
type PhotosQueryResponse struct {
	Data     cloudkit.CKQueryResponse
	Metadata *BytesResponse
}

// PhotosIndexing checks the primary photo library with operation-local credentials.
func (client *Client) PhotosIndexing(ctx context.Context, auth RequestContext) (*PhotosQueryResponse, error) {
	zone := cloudkit.CKZoneIDReq{ZoneName: protocol.PhotosPhotoPrimaryZoneNameValue,
		ZoneType: nil, OwnerRecordName: nil, AdditionalProperties: nil}
	zone.ZoneType.Set(protocol.PhotosPhotoPrimaryZoneTypeValue)

	if auth.PhotoZone != nil {
		zone = *auth.PhotoZone
	}
	return client.photosZoneIndexing(ctx, auth, zone, auth.PhotoShared)
}

// PhotosLibraryIndexing initializes one discovered library in its advertised database scope.
func (client *Client) PhotosLibraryIndexing(ctx context.Context, auth RequestContext,
	identity cloudkit.CKZoneID, shared bool,
) (*PhotosQueryResponse, error) {
	zone := cloudkit.CKZoneIDReq{ZoneName: identity.ZoneName, ZoneType: nil,
		OwnerRecordName: nil, AdditionalProperties: identity.AdditionalProperties}

	value, err := identity.ZoneType.Get()
	if err == nil {
		zone.ZoneType.Set(value)
	}

	value, err = identity.OwnerRecordName.Get()
	if err == nil {
		zone.OwnerRecordName.Set(value)
	}

	return client.photosZoneIndexing(ctx, auth, zone, shared)
}

func (client *Client) photosZoneIndexing(ctx context.Context, auth RequestContext,
	zone cloudkit.CKZoneIDReq, shared bool,
) (*PhotosQueryResponse, error) {
	identity, err := referenceJSONFields(zone, []string{protocol.PhotosCKZoneIDReqZoneName,
		protocol.PhotosCKZoneIDReqZoneType, protocol.PhotosCKZoneIDReqOwnerRecordName})
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	body := fmt.Sprintf("{%q: {%q: %q}, %q: %s, %q: %d}", protocol.PhotosCKQueryRequestQuery,
		protocol.PhotosCKQueryObjectRecordType, protocol.PhotosPhotoIndexingRecordTypeValue,
		protocol.PhotosCKQueryRequestZoneID, identity, protocol.PhotosCKQueryRequestResultsLimit, 1)

	return client.photosScopedQueryBytes(ctx, auth, body, shared)
}

func (client *Client) photosQueryBytes(
	ctx context.Context, auth RequestContext, body string,
) (*PhotosQueryResponse, error) {
	return client.photosScopedQueryBytes(ctx, auth, body, auth.PhotoShared)
}

func (client *Client) photosScopedQueryBytes(ctx context.Context, auth RequestContext,
	body string, shared bool,
) (*PhotosQueryResponse, error) {
	request, err := photosQueryRequest(auth, body, shared)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	err = validateOrigin(auth.Origin)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	request = request.WithContext(ctx)
	request.Header = auth.Headers.Clone()
	request.Header.Set(protocol.HTTPContentTypeName, protocol.PhotosMediaApplicationJson)

	if request.Header.Get(protocol.AcceptName) == "" {
		request.Header.Set(protocol.AcceptName, string(accountapi.AcceptAsterisk))
	}

	request.URL.RawQuery = orderedAccountQuery(auth.Params) + "&" +
		queryPart(protocol.PhotosRemapEnumsName, string(photosapi.PhotosQueryRecordsParamsRemapEnumsTrue)) + "&" +
		queryPart(protocol.PhotosGetCurrentSyncTokenName, string(photosapi.PhotosQueryRecordsParamsGetCurrentSyncTokenTrue))

	response, err := client.readPrepared(request, successfulContent, auth.Cookies)
	if err != nil {
		return nil, err
	}
	// Photos and Reminders use the same pinned CloudKit record and field unions.
	data, err := decodeReminderSyncQuery(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &PhotosQueryResponse{Data: data, Metadata: response}, nil
}

func photosQueryRequest(auth RequestContext, body string, shared bool) (*http.Request, error) {
	if shared {
		params := new(photosapi.PhotosQuerySharedRecordsParams)
		params.ClientId = auth.Params.ClientId
		params.Dsid = auth.Params.Dsid
		params.RemapEnums = photosapi.PhotosQuerySharedRecordsParamsRemapEnumsTrue
		params.GetCurrentSyncToken = photosapi.PhotosQuerySharedRecordsParamsGetCurrentSyncTokenTrue

		request, err := photosapi.NewPhotosQuerySharedRecordsRequestWithBody(auth.Origin, params,
			protocol.PhotosMediaApplicationJson, bytes.NewBufferString(body))
		if err != nil {
			return nil, fmt.Errorf("construct shared photo query: %w", err)
		}

		return request, nil
	}

	params := new(photosapi.PhotosQueryRecordsParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = photosapi.PhotosQueryRecordsParamsRemapEnumsTrue
	params.GetCurrentSyncToken = photosapi.PhotosQueryRecordsParamsGetCurrentSyncTokenTrue

	request, err := photosapi.NewPhotosQueryRecordsRequestWithBody(auth.Origin, params,
		protocol.PhotosMediaApplicationJson, bytes.NewBufferString(body))
	if err != nil {
		return nil, fmt.Errorf("construct private photo query: %w", err)
	}

	return request, nil
}
