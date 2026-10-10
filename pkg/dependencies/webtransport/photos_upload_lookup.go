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

	input := new(cloudkit.CKLookupRequest)
	input.Records = []cloudkit.CKLookupDescriptor{
		{RecordName: masterName, AdditionalProperties: nil}, {RecordName: assetName, AdditionalProperties: nil},
	}
	input.ZoneID = *zone
	desired := photoUploadDesiredKeys()

	keys := make([]string, len(desired))
	for index, key := range desired {
		keys[index] = string(key)
	}

	input.DesiredKeys.Set(keys)

	body, err := referenceJSONFields(input, []string{protocol.PhotosUploadCKLookupRequestRecords,
		protocol.PhotosUploadCKLookupRequestZoneID, protocol.PhotosUploadCKLookupRequestDesiredKeys})
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	body, err = photosOrderedZone(body, *zone)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

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

func photoUploadLookupRequest(auth RequestContext, body []byte) (*http.Request, error) {
	var (
		request *http.Request
		err     error
	)

	if auth.PhotoShared {
		request, err = photosuploadapi.NewPhotosHydrateUploadedAssetSharedRequestWithBody(auth.Origin, nil,
			jsonMedia(), bytes.NewReader(body))
	} else {
		request, err = photosuploadapi.NewPhotosHydrateUploadedAssetRequestWithBody(auth.Origin, nil,
			jsonMedia(), bytes.NewReader(body))
	}

	if err != nil {
		return nil, fmt.Errorf("construct uploaded photo lookup: %w", err)
	}

	return request, nil
}
