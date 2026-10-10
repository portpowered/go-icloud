package webtransport

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/photosapi"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/sharedphotosapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/sharedphotos"
)

// SharedAlbumsResponse retains the shared stream discovery response.
type SharedAlbumsResponse struct {
	Data     sharedphotos.SharedAlbumsResponse
	Metadata *BytesResponse
}

// SharedCountResponse retains one stream's count and response evidence.
type SharedCountResponse struct {
	Data     sharedphotos.SharedCountResponse
	Metadata *BytesResponse
}

// SharedAssetsResponse contains only records recognized by the legacy parser.
type SharedAssetsResponse struct {
	Records  []cloudkit.CKRecord
	Metadata *BytesResponse
}

// PhotosSharedAlbums discovers streams using the account-provided origin.
func (client *Client) PhotosSharedAlbums(ctx context.Context, auth RequestContext) (*SharedAlbumsResponse, error) {
	body, err := referenceJSON(sharedphotos.SharedAlbumsRequest{})
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	params := new(sharedphotosapi.SharedAlbumsParams)
	params.ClientId, params.Dsid = auth.Params.ClientId, auth.Params.Dsid
	params.RemapEnums = sharedphotosapi.SharedAlbumsParamsRemapEnumsTrue
	params.GetCurrentSyncToken = sharedphotosapi.SharedAlbumsParamsGetCurrentSyncTokenTrue

	request, err := sharedphotosapi.NewSharedAlbumsRequestWithBody(auth.Origin, auth.Params.Dsid, params,
		protocol.SharedPhotosMediaPlainText, bytes.NewReader(body))

	response, err := client.readSharedPhotos(ctx, auth, request, err)
	if err != nil {
		return nil, err
	}

	var data sharedphotos.SharedAlbumsResponse

	err = decodeSharedAlbums(response.Body, &data)
	if err != nil {
		return nil, responseFailure(Decode, errPhotosCountShape, response)
	}

	return &SharedAlbumsResponse{Data: data, Metadata: response}, nil
}

// PhotosSharedCount reads the count at a discovered album location.
func (client *Client) PhotosSharedCount(ctx context.Context, auth RequestContext,
	album sharedphotos.SharedAlbum,
) (*SharedCountResponse, error) {
	body, err := referenceJSON(sharedphotos.SharedCountRequest{Albumguid: album.Albumguid})
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	origin, location, err := sharedAlbumLocation(album.Albumlocation)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	auth.Origin = origin
	params := new(sharedphotosapi.SharedCountParams)
	params.ClientId, params.Dsid = auth.Params.ClientId, auth.Params.Dsid
	params.RemapEnums = sharedphotosapi.SharedCountParamsRemapEnumsTrue
	params.GetCurrentSyncToken = sharedphotosapi.SharedCountParamsGetCurrentSyncTokenTrue

	request, err := sharedphotosapi.NewSharedCountRequestWithBody(location, params,
		protocol.SharedPhotosMediaPlainText, bytes.NewReader(body))

	response, err := client.readSharedPhotos(ctx, auth, request, err)
	if err != nil {
		return nil, err
	}

	var (
		data   sharedphotos.SharedCountResponse
		fields map[string]json.RawMessage
	)

	if json.Unmarshal(response.Body, &fields) != nil ||
		fields[protocol.SharedPhotosSharedCountResponseAlbumassetcount] == nil ||
		string(fields[protocol.SharedPhotosSharedCountResponseAlbumassetcount]) == jsonNullValue ||
		json.Unmarshal(response.Body, &data) != nil || data.Albumassetcount < 0 {
		return nil, responseFailure(Decode, errPhotosCountShape, response)
	}

	return &SharedCountResponse{Data: data, Metadata: response}, nil
}

func decodeSharedAlbums(body []byte, data *sharedphotos.SharedAlbumsResponse) error {
	var raw map[string]json.RawMessage

	err := json.Unmarshal(body, &raw)
	if err != nil {
		return errPhotosCountShape
	}

	var albums []map[string]json.RawMessage

	if json.Unmarshal(raw[protocol.SharedPhotosSharedAlbumsResponseAlbums], &albums) != nil || albums == nil {
		return errPhotosCountShape
	}

	for _, album := range albums {
		if !reminderModelRequired(reflect.TypeFor[sharedphotos.SharedAlbum](), album) {
			return errPhotosCountShape
		}

		var attributes map[string]json.RawMessage

		if json.Unmarshal(album[protocol.SharedPhotosSharedAlbumAttributes], &attributes) != nil ||
			!reminderModelRequired(reflect.TypeFor[sharedphotos.SharedAlbumAttributes](), attributes) {
			return errPhotosCountShape
		}
	}

	err = json.Unmarshal(body, data)
	if err != nil {
		return errPhotosCountShape
	}

	return nil
}

// PhotosSharedAssets reads a page with cumulative limit and offset strings.
func (client *Client) PhotosSharedAssets(ctx context.Context, auth RequestContext,
	album sharedphotos.SharedAlbum, offset, pageSize int64,
) (*SharedAssetsResponse, error) {
	body, err := referenceJSONFields(sharedphotos.SharedAssetsRequest{Albumguid: album.Albumguid,
		Albumctag: album.Albumctag, Limit: strconv.FormatInt(offset+pageSize, 10), Offset: strconv.FormatInt(offset, 10)},
		[]string{protocol.SharedPhotosSharedAssetsRequestAlbumguid, protocol.SharedPhotosSharedAssetsRequestAlbumctag,
			protocol.SharedPhotosSharedAssetsRequestLimit, protocol.SharedPhotosSharedAssetsRequestOffset})
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	origin, location, err := sharedAlbumLocation(album.Albumlocation)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	auth.Origin = origin
	params := new(sharedphotosapi.SharedAssetsParams)
	params.ClientId, params.Dsid = auth.Params.ClientId, auth.Params.Dsid
	params.RemapEnums = sharedphotosapi.SharedAssetsParamsRemapEnumsTrue
	params.GetCurrentSyncToken = sharedphotosapi.SharedAssetsParamsGetCurrentSyncTokenTrue

	request, err := sharedphotosapi.NewSharedAssetsRequestWithBody(location, params,
		protocol.SharedPhotosMediaPlainText, bytes.NewReader(body))

	response, err := client.readSharedPhotos(ctx, auth, request, err)
	if err != nil {
		return nil, err
	}

	records, err := decodeSharedRecords(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &SharedAssetsResponse{Records: records, Metadata: response}, nil
}

func decodeSharedRecords(body []byte) ([]cloudkit.CKRecord, error) {
	var envelope map[string]json.RawMessage
	if json.Unmarshal(body, &envelope) != nil || envelope == nil {
		return nil, errPhotosCountShape
	}
	var rawRecords []json.RawMessage
	if json.Unmarshal(envelope[protocol.SharedPhotosSharedRecordsResponseRecords], &rawRecords) != nil {
		return []cloudkit.CKRecord{}, nil
	}
	records := []cloudkit.CKRecord{}
	for _, raw := range rawRecords {
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil || fields == nil {
			continue
		}
		var kind cloudkit.PhotoRecordKind
		_ = json.Unmarshal(fields[protocol.PhotosCKRecordRecordType], &kind)
		if kind != cloudkit.CPLAsset && kind != cloudkit.CPLMaster {
			continue
		}
		if kind == cloudkit.CPLAsset && !sharedAssetReference(fields[protocol.PhotosCKRecordFields]) {
			continue
		}
		if kind == cloudkit.CPLMaster {
			var name string
			if json.Unmarshal(fields[protocol.PhotosCKRecordRecordName], &name) != nil {
				continue
			}
		}
		var record cloudkit.CKRecord
		if json.Unmarshal(raw, &record) != nil {
			return nil, errPhotosCountShape
		}
		records = append(records, record)
	}
	return records, nil
}

func sharedAssetReference(raw json.RawMessage) bool {
	var fields, wrapper, reference map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil ||
		json.Unmarshal(fields[protocol.SharedPhotosSharedAssetFieldsMasterRef], &wrapper) != nil || wrapper == nil ||
		json.Unmarshal(wrapper[protocol.PhotosCKPassthroughFieldValue], &reference) != nil || reference == nil {
		return false
	}
	var name string
	return json.Unmarshal(reference[protocol.PhotosCKReferenceRecordName], &name) == nil
}

func (client *Client) readSharedPhotos(ctx context.Context, auth RequestContext,
	request *http.Request, construction error,
) (*BytesResponse, error) {
	if construction != nil {
		return nil, failure(Configuration, construction, nil, nil)
	}

	return client.read(ctx, auth, request, "&"+
		queryPart(protocol.PhotosRemapEnumsName, string(photosapi.PhotosQueryRecordsParamsRemapEnumsTrue))+"&"+
		queryPart(protocol.PhotosGetCurrentSyncTokenName, string(photosapi.PhotosQueryRecordsParamsGetCurrentSyncTokenTrue)))
}

func sharedAlbumLocation(location string) (string, string, error) {
	parsed, err := url.Parse(location)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		!strings.HasSuffix(location, "/") ||
		validateOrigin(parsed.Scheme+"://"+parsed.Host) != nil {
		return "", "", errPhotoContentURL
	}

	return parsed.Scheme + "://" + parsed.Host, parsed.String(), nil
}
