package webtransport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/photosuploadapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/photosupload"
)

var errPhotoUploadPayload = errors.New("unexpected Photos upload response")

// PhotosReserveUploads reserves byte lengths under caller-supplied identities.
func (client *Client) PhotosReserveUploads(ctx context.Context, auth RequestContext,
	payload photosupload.PhotosCreateUploadUrlRequest,
	cloudKitFlags bool,
) (*BytesResponse, photosupload.PhotosUploadReservationResponse, error) {
	var data photosupload.PhotosUploadReservationResponse

	body, err := referenceJSON(payload)
	if err != nil {
		return nil, data, failure(Configuration, err, nil, nil)
	}

	params := new(photosuploadapi.PhotosCreateUploadUrlParams)
	params.ClientId, params.Dsid = &auth.Params.ClientId, &auth.Params.Dsid
	request, err := photosuploadapi.NewPhotosCreateUploadUrlRequestWithBody(auth.Origin, params,
		protocol.PhotosMediaApplicationJson, bytes.NewReader(body))

	response, err := client.readPhotoUpload(ctx, auth, request, err, cloudKitFlags)
	if err != nil {
		return nil, data, err
	}

	err = decodePhotoUploadObject(response, &data)
	if err != nil {
		return nil, data, err
	}

	if !validPhotoReservationURLs(response.Body) {
		return nil, data, responseFailure(Decode, errPhotoUploadPayload, response)
	}

	return response, data, nil
}

func validPhotoReservationURLs(body []byte) bool {
	var fields map[string]json.RawMessage

	if json.Unmarshal(body, &fields) != nil {
		return false
	}

	raw := fields[protocol.PhotosUploadReservationResponseUploadUrls]
	if raw == nil {
		return true
	}

	var urls map[string]json.RawMessage

	if json.Unmarshal(raw, &urls) != nil || urls == nil {
		return false
	}

	for _, value := range urls {
		if bytes.Equal(value, []byte("null")) {
			return false
		}
	}

	return true
}

// PhotosRegisterUploads registers previously stored receipts without retrying writes.
func (client *Client) PhotosRegisterUploads(ctx context.Context, auth RequestContext,
	payload photosupload.PhotosPutAssetRequest,
	cloudKitFlags bool,
) (*BytesResponse, photosupload.PhotosPutAssetResults, error) {
	var data photosupload.PhotosPutAssetResults

	body, err := orderedPhotoRegistration(payload)
	if err != nil {
		return nil, data, failure(Configuration, err, nil, nil)
	}

	params := new(photosuploadapi.PhotosPutAssetParams)
	params.ClientId, params.Dsid = &auth.Params.ClientId, &auth.Params.Dsid
	request, err := photosuploadapi.NewPhotosPutAssetRequestWithBody(auth.Origin, params,
		protocol.PhotosMediaApplicationJson, bytes.NewReader(body))

	response, err := client.readPhotoUpload(ctx, auth, request, err, cloudKitFlags)
	if err != nil {
		return nil, data, err
	}

	var entries []json.RawMessage

	err = json.Unmarshal(response.Body, &entries)
	if err == nil {
		for _, entry := range entries {
			if bytes.Equal(entry, []byte("null")) {
				err = errPhotoUploadPayload

				break
			}
		}
	}

	if err == nil {
		err = json.Unmarshal(response.Body, &data)
	}

	if err != nil || data == nil {
		return nil, data, responseFailure(Decode, errors.Join(err, errPhotoUploadPayload), response)
	}

	return response, data, nil
}

// PhotosUploadStatuses reports ingest progress independently of CloudKit indexing.
func (client *Client) PhotosUploadStatuses(ctx context.Context, auth RequestContext,
	jobs []string,
) (*BytesResponse, photosupload.PhotosUploadStatusEntries, error) {
	var data photosupload.PhotosUploadStatusEntries

	body, err := referenceJSON(photosupload.PhotosUploadStatusRequest{UploadJobIds: jobs})
	if err != nil {
		return nil, data, failure(Configuration, err, nil, nil)
	}

	params := new(photosuploadapi.PhotosUploadStatusParams)
	params.ClientId, params.Dsid = &auth.Params.ClientId, &auth.Params.Dsid
	request, err := photosuploadapi.NewPhotosUploadStatusRequestWithBody(auth.Origin, params,
		protocol.PhotosMediaApplicationJson, bytes.NewReader(body))

	response, err := client.readPhotoUpload(ctx, auth, request, err, false)
	if err != nil {
		return nil, data, err
	}

	err = decodePhotoUploadObject(response, &data)
	if err == nil {
		var entries map[string]json.RawMessage

		_ = json.Unmarshal(response.Body, &entries)

		for _, entry := range entries {
			if bytes.Equal(entry, []byte("null")) {
				return nil, data, responseFailure(Decode, errPhotoUploadPayload, response)
			}
		}
	}

	return response, data, err
}

func (client *Client) readPhotoUpload(ctx context.Context, auth RequestContext,
	request *http.Request, constructionErr error,
	cloudKitFlags bool,
) (*BytesResponse, error) {
	if constructionErr != nil {
		return nil, failure(Configuration, constructionErr, nil, nil)
	}

	return client.readWithPolicy(ctx, auth, request, photoUploadQuerySuffix(cloudKitFlags), successfulContent, false)
}

func photoUploadQuerySuffix(cloudKitFlags bool) string {
	if !cloudKitFlags {
		return ""
	}

	remap := string(photosuploadapi.PhotosUploadremapEnumsPhotosUploadRemapEnumsTrue)
	sync := string(photosuploadapi.PhotosUploadgetCurrentSyncTokenPhotosUploadCurrentSyncTokenTrue)
	suffix := "&" + queryPart(protocol.PhotosUploadremapEnumsName, remap) +
		"&" + queryPart(protocol.PhotosUploadgetCurrentSyncTokenName, sync)

	return suffix
}

func decodePhotoUploadObject(response *BytesResponse, destination any) error {
	var object map[string]json.RawMessage

	err := json.Unmarshal(response.Body, &object)
	if err != nil || object == nil {
		return responseFailure(Decode, errors.Join(err, errPhotoUploadPayload), response)
	}

	err = json.Unmarshal(response.Body, destination)
	if err != nil {
		return responseFailure(Decode, err, response)
	}

	return nil
}

// PhotosUploadBytes sends the remaining caller-owned bytes to an HTTPS reservation.
// The caller retains ownership of the reader and its cursor.
func (client *Client) PhotosUploadBytes(ctx context.Context, auth RequestContext,
	targetURL string, content io.ReadSeeker,
) (*BytesResponse, photosupload.PhotosSingleFileUpload, error) {
	var receipt photosupload.PhotosSingleFileUpload

	request, err := photoUploadBytesRequest(targetURL, content)
	if err != nil {
		return nil, receipt, err
	}

	request.Header = auth.Headers.Clone()

	response, err := client.readPrepared(request.WithContext(ctx), successfulContent, auth.Cookies)
	if err != nil {
		return nil, receipt, err
	}

	data, err := decodePhotoUploadReceipt(response)

	return response, data, err
}

func photoUploadBytesRequest(targetURL string, content io.ReadSeeker) (*http.Request, error) {
	target, err := url.Parse(targetURL)
	if err != nil || target.Scheme != "https" || target.Host == "" || target.User != nil || target.Fragment != "" {
		return nil, failure(Configuration, errPhotoContentURL, nil, nil)
	}

	size, err := uploadFileSize(content)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	position, err := content.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, failure(Configuration, fmt.Errorf("read upload position: %w", err), nil, nil)
	}

	request, err := photosuploadapi.NewPhotosSendUploadBytesRequestWithBody(target.Scheme+"://"+target.Host,
		target.EscapedPath(), nil, "", io.LimitReader(content, max(size-position, 0)))
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	request.URL = target

	request.ContentLength = max(size-position, 0)
	if request.ContentLength == 0 {
		request.ContentLength = -1
		request.TransferEncoding = []string{string(photosupload.Chunked)}
	}

	return request, nil
}

func decodePhotoUploadReceipt(response *BytesResponse) (photosupload.PhotosSingleFileUpload, error) {
	var (
		receipt photosupload.PhotosSingleFileUpload
		data    photosupload.PhotosSingleFileUploadResponse
	)

	err := decodePhotoUploadObject(response, &data)
	if err != nil {
		return receipt, err
	}

	var fields map[string]json.RawMessage

	err = json.Unmarshal(response.Body, &fields)
	if err != nil || fields[protocol.PhotosUploadPhotosSingleFileUploadResponseSingleFile] == nil ||
		bytes.Equal(fields[protocol.PhotosUploadPhotosSingleFileUploadResponseSingleFile], []byte("null")) {
		return receipt, responseFailure(Decode, errPhotoUploadPayload, response)
	}

	return data.SingleFile, nil
}
