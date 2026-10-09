package webtransport

import (
	"bytes"
	"context"
	"fmt"
	"github.com/portpowered/go-icloud/internal/accountapi"
	"github.com/portpowered/go-icloud/internal/photosapi"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
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
	identity, err := referenceJSON(zone)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}
	body := fmt.Sprintf("{%q: {%q: %q}, %q: %s, %q: %d}", protocol.PhotosCKQueryRequestQuery,
		protocol.PhotosCKQueryObjectRecordType, protocol.PhotosPhotoIndexingRecordTypeValue,
		protocol.PhotosCKQueryRequestZoneID, identity, protocol.PhotosCKQueryRequestResultsLimit, 1)
	params := new(photosapi.PhotosQueryRecordsParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = photosapi.PhotosQueryRecordsParamsRemapEnumsTrue
	params.GetCurrentSyncToken = photosapi.PhotosQueryRecordsParamsGetCurrentSyncTokenTrue
	request, err := photosapi.NewPhotosQueryRecordsRequestWithBody(auth.Origin, params,
		protocol.PhotosMediaApplicationJson, bytes.NewBufferString(body))
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
		queryPart(protocol.PhotosRemapEnumsName, string(params.RemapEnums)) + "&" +
		queryPart(protocol.PhotosGetCurrentSyncTokenName, string(params.GetCurrentSyncToken))
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
