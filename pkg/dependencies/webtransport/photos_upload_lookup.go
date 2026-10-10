package webtransport

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/photosuploadapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// PhotosHydrateUpload looks up the registered master and asset in the selected zone.
func (client *Client) PhotosHydrateUpload(ctx context.Context, auth RequestContext,
	masterName, assetName string,
) (*ReminderLookupResponse, error) {
	zone := new(cloudkit.CKZoneIDReq)
	zone.ZoneName = protocol.PhotosPhotoPrimaryZoneNameValue
	zone.ZoneType.Set(protocol.PhotosPhotoPrimaryZoneTypeValue)

	if auth.PhotoZone != nil {
		zone = auth.PhotoZone
	}

	identity, err := referenceJSONFields(zone, []string{protocol.PhotosCKZoneIDReqZoneName,
		protocol.PhotosCKZoneIDReqZoneType, protocol.PhotosCKZoneIDReqOwnerRecordName})
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	descriptors := []cloudkit.CKLookupDescriptor{
		{RecordName: masterName, AdditionalProperties: nil}, {RecordName: assetName, AdditionalProperties: nil},
	}

	records, err := referenceJSON(descriptors)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	keys, err := referenceJSON(photoUploadDesiredKeys())
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	body := fmt.Sprintf("{%q: %s, %q: %s, %q: %s}", protocol.PhotosUploadCKLookupRequestRecords, records,
		protocol.PhotosUploadCKLookupRequestZoneID, identity, protocol.PhotosUploadCKLookupRequestDesiredKeys, keys)

	request, err := photoUploadLookupRequest(auth, body)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	response, err := client.readPhotoUpload(ctx, auth, request, nil, true)
	if err != nil {
		return nil, err
	}

	data, err := decodeReminderLookup(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &ReminderLookupResponse{Data: data, Metadata: response}, nil
}

func photoUploadLookupRequest(auth RequestContext, body string) (*http.Request, error) {
	var (
		request *http.Request
		err     error
	)

	if auth.PhotoShared {
		request, err = photosuploadapi.NewPhotosHydrateUploadedAssetSharedRequestWithBody(auth.Origin, nil,
			jsonMedia(), bytes.NewBufferString(body))
	} else {
		request, err = photosuploadapi.NewPhotosHydrateUploadedAssetRequestWithBody(auth.Origin, nil,
			jsonMedia(), bytes.NewBufferString(body))
	}

	if err != nil {
		return nil, fmt.Errorf("construct uploaded photo lookup: %w", err)
	}

	return request, nil
}
