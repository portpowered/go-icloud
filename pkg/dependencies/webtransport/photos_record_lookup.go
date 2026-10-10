package webtransport

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/photosapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// LookupPhotosRecords reads explicit record names in a selected CloudKit library.
func (client *Client) LookupPhotosRecords(
	ctx context.Context,
	auth RequestContext,
	names []string,
) (*ReminderLookupResponse, error) {
	body, err := photosRecordsLookupBody(auth, names)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	request, err := photosRecordLookupRequest(auth, body)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	response, err := client.readPhotosContainerRequest(ctx, auth, request)
	if err != nil {
		return nil, err
	}

	data, err := decodeReminderLookup(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &ReminderLookupResponse{Data: data, Metadata: response}, nil
}
func photosRecordLookupRequest(auth RequestContext, body []byte) (*http.Request, error) {
	if auth.PhotoShared {
		params := photosSharedLookupRecordsParams(auth)

		request, err := photosapi.NewPhotosSharedLookupRecordsRequestWithBody(
			auth.Origin,
			params,
			jsonMedia(),
			bytes.NewReader(body),
		)
		if err != nil {
			return nil, fmt.Errorf("construct shared photo record lookup: %w", err)
		}

		return request, nil
	}

	params := photosLookupRecordsParams(auth)

	request, err := photosapi.NewPhotosLookupRecordsRequestWithBody(
		auth.Origin,
		params,
		jsonMedia(),
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("construct photo record lookup: %w", err)
	}

	return request, nil
}

func photosSharedLookupRecordsParams(auth RequestContext) *photosapi.PhotosSharedLookupRecordsParams {
	params := new(photosapi.PhotosSharedLookupRecordsParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = photosapi.PhotosSharedLookupRecordsParamsRemapEnumsTrue
	params.GetCurrentSyncToken = photosapi.PhotosSharedLookupRecordsParamsGetCurrentSyncTokenTrue

	return params
}

func photosLookupRecordsParams(auth RequestContext) *photosapi.PhotosLookupRecordsParams {
	params := new(photosapi.PhotosLookupRecordsParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = photosapi.PhotosLookupRecordsParamsRemapEnumsTrue
	params.GetCurrentSyncToken = photosapi.PhotosLookupRecordsParamsGetCurrentSyncTokenTrue

	return params
}

func photosRecordsLookupBody(auth RequestContext, names []string) ([]byte, error) {
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

	records := make([]cloudkit.CKLookupDescriptor, 0, len(names))

	for _, name := range names {
		item := new(cloudkit.CKLookupDescriptor)
		item.RecordName = name
		records = append(records, *item)
	}

	input := new(cloudkit.CKLookupRequest)
	input.Records = records
	input.ZoneID = zone

	body, err := referenceJSON(input)
	if err != nil {
		return nil, fmt.Errorf("encode photo records lookup: %w", err)
	}

	original, err := referenceJSON(zone)
	if err != nil {
		return nil, fmt.Errorf("encode photo records lookup: %w", err)
	}

	ordered, err := referenceJSONFields(
		zone,
		[]string{
			protocol.PhotosCKZoneIDReqZoneName,
			protocol.PhotosCKZoneIDReqZoneType,
			protocol.PhotosCKZoneIDReqOwnerRecordName,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("encode photo records lookup: %w", err)
	}

	body = bytes.ReplaceAll(body, original, ordered)

	return body, nil
}
