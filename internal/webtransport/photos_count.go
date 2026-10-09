package webtransport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

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

	body, err := photosCountJSON(body)
	if err != nil {
		return data, err
	}

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

	return data, nil
}

// Source JSON decoding rounds decimal and exponent numbers to binary64 before
// its integer model validates them. Keep plain JSON integers exact, and retain
// the original HTTP bytes separately as response evidence.
func photosCountJSON(body []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()

	var value any

	err := decoder.Decode(&value)
	if err != nil {
		return nil, fmt.Errorf("decode photo count numbers: %w", err)
	}

	// Unmarshal also rejects trailing JSON before normalization replaces bytes.
	var complete json.RawMessage

	err = json.Unmarshal(body, &complete)
	if err != nil {
		return nil, fmt.Errorf("decode photo count document: %w", err)
	}

	path := []string{protocol.PhotosCountResponseBatch, protocol.PhotosCountResponseBatchRecords,
		protocol.PhotosCountRecordFields, protocol.PhotosCountFieldsItemCount, protocol.PhotosCountFieldValue}

	normalized, err := json.Marshal(photosCountNumbers(value, path))
	if err != nil {
		return nil, fmt.Errorf("normalize photo count numbers: %w", err)
	}

	return normalized, nil
}

func photosCountNumbers(value any, path []string) any {
	if len(path) == 0 {
		return photosCountNumber(value)
	}

	switch typed := value.(type) {
	case []any:
		for index, item := range typed {
			typed[index] = photosCountNumbers(item, path)
		}
	case map[string]any:
		key := path[0]
		if item, exists := typed[key]; exists {
			typed[key] = photosCountNumbers(item, path[1:])
		}
	}

	return value
}

func photosCountNumber(value any) any {
	number, ok := value.(json.Number)
	if !ok || !strings.ContainsAny(string(number), ".eE") {
		return value
	}

	converted, err := strconv.ParseFloat(string(number), 64)
	if err == nil || errors.Is(err, strconv.ErrRange) {
		return converted
	}

	return value
}
