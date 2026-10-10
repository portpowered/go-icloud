package webtransport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/accountapi"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/photosapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// PhotosZonesResponse retains discovered identities and the full wire response.
type PhotosZonesResponse struct {
	Data     cloudkit.CKZoneListResponse
	Metadata *BytesResponse
}

// PhotosLibraryZones reads one scope so the caller can initialize libraries before advancing discovery.
func (client *Client) PhotosLibraryZones(ctx context.Context, auth RequestContext,
	shared bool,
) (*PhotosZonesResponse, error) {
	request, err := photosZoneRequest(auth, shared)
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

	data, err := decodeReminderZones(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &PhotosZonesResponse{Data: data, Metadata: response}, nil
}

func photosZoneRequest(auth RequestContext, shared bool) (*http.Request, error) {
	if shared {
		params := new(photosapi.PhotosListSharedZonesParams)
		params.ClientId = auth.Params.ClientId
		params.Dsid = auth.Params.Dsid
		params.RemapEnums = photosapi.PhotosListSharedZonesParamsRemapEnumsTrue
		params.GetCurrentSyncToken = photosapi.PhotosListSharedZonesParamsGetCurrentSyncTokenTrue

		request, err := photosapi.NewPhotosListSharedZonesRequestWithBody(auth.Origin, params,
			protocol.PhotosMediaApplicationJson, bytes.NewBufferString("{}"))
		if err != nil {
			return nil, fmt.Errorf("construct shared photo zones request: %w", err)
		}

		return request, nil
	}

	params := new(photosapi.PhotosListPrivateZonesParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = photosapi.PhotosListPrivateZonesParamsRemapEnumsTrue
	params.GetCurrentSyncToken = photosapi.PhotosListPrivateZonesParamsGetCurrentSyncTokenTrue

	request, err := photosapi.NewPhotosListPrivateZonesRequestWithBody(auth.Origin, params,
		protocol.PhotosMediaApplicationJson, bytes.NewBufferString("{}"))
	if err != nil {
		return nil, fmt.Errorf("construct private photo zones request: %w", err)
	}

	return request, nil
}

// SuppressedPhotosResponse exposes response evidence for Source-tolerated shared discovery failures.
func SuppressedPhotosResponse(err error) *BytesResponse {
	var failure *ResponseError
	if !errors.As(err, &failure) || (failure.Stage != Provider && failure.Stage != Decode) {
		return nil
	}

	return &BytesResponse{Status: failure.Status, Body: failure.Body, Headers: failure.Headers,
		CookieScopeURL: failure.CookieScopeURL}
}
