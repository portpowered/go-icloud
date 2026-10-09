package webtransport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/portpowered/go-icloud/internal/photosapi"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

var errPhotosCountShape = errors.New("invalid photo count response")

// PhotosCountResponse retains a completely validated count reply and its evidence.
type PhotosCountResponse struct {
	Data     cloudkit.PhotosCountResponse
	Metadata *BytesResponse
}

// PhotosAlbumCount reads the internal Hyperion count index for one album.
func (client *Client) PhotosAlbumCount(
	ctx context.Context, auth RequestContext, index string,
) (*PhotosCountResponse, error) {
	zone := cloudkit.CKZoneIDReq{ZoneName: protocol.PhotosPhotoPrimaryZoneNameValue,
		ZoneType: nil, OwnerRecordName: nil, AdditionalProperties: nil}
	zone.ZoneType.Set(protocol.PhotosPhotoPrimaryZoneTypeValue)

	filter := cloudkit.PhotosCountFilter{FieldName: cloudkit.IndexCountID,
		Comparator: cloudkit.PhotosCountFilterComparatorIN,
		FieldValue: cloudkit.PhotosCountStringList{Type: cloudkit.PhotosCountStringListTypeSTRINGLIST,
			Value: []string{index}}}
	query := cloudkit.PhotosCountQuery{RecordType: cloudkit.HyperionIndexCountLookup,
		FilterBy: filter}
	payload := cloudkit.PhotosCountRequest{Batch: []cloudkit.PhotosCountRequestBatch{{
		ResultsLimit: cloudkit.PhotosCountRequestBatchResultsLimitN1, Query: query,
		ZoneWide: cloudkit.True, ZoneID: zone}}}

	body, err := referenceJSON(payload)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	params := new(photosapi.PhotosCountAlbumsParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = photosapi.PhotosCountAlbumsParamsRemapEnumsTrue
	params.GetCurrentSyncToken = photosapi.PhotosCountAlbumsParamsGetCurrentSyncTokenTrue

	request, err := photosapi.NewPhotosCountAlbumsRequestWithBody(auth.Origin, params,
		protocol.PhotosMediaPlainText, bytes.NewReader(body))
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	err = validateOrigin(auth.Origin)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	request = request.WithContext(ctx)
	request.Header = auth.Headers.Clone()
	request.Header.Set(protocol.HTTPContentTypeName, protocol.PhotosMediaPlainText)
	request.URL.RawQuery = orderedAccountQuery(auth.Params) + "&" +
		queryPart(protocol.PhotosRemapEnumsName, string(params.RemapEnums)) + "&" +
		queryPart(protocol.PhotosGetCurrentSyncTokenName, string(params.GetCurrentSyncToken))

	response, err := client.readPrepared(request, successfulContent, auth.Cookies)
	if err != nil {
		return nil, err
	}

	data, err := decodePhotosCount(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &PhotosCountResponse{Data: data, Metadata: response}, nil
}

func decodePhotosCount(body []byte) (cloudkit.PhotosCountResponse, error) {
	var data cloudkit.PhotosCountResponse

	fields, err := accountFields(body)
	if err != nil {
		return data, err
	}

	if !reminderModelRequired(reflect.TypeFor[cloudkit.PhotosCountResponse](), fields) {
		return data, errPhotosCountShape
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return data, fmt.Errorf("decode photo count: %w", err)
	}

	if data.Batch == nil {
		return data, nil
	}

	for _, batch := range *data.Batch {
		if batch.Records == nil {
			continue
		}

		for _, record := range *batch.Records {
			raw, encodeErr := json.Marshal(record.Fields.ItemCount.Value)
			if encodeErr != nil || !reminderSyncInteger(raw) {
				return data, errPhotosCountShape
			}
		}
	}

	return data, nil
}
