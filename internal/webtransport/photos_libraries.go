package webtransport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/portpowered/go-icloud/internal/accountapi"
	"github.com/portpowered/go-icloud/internal/photosapi"
	"github.com/portpowered/go-icloud/internal/protocol"
)

// PhotosLibraryZones reads private then shared zone discovery with operation-local credentials.
func (client *Client) PhotosLibraryZones(ctx context.Context, auth RequestContext) ([]*BytesResponse, error) {
	responses := []*BytesResponse{}

	for _, shared := range []bool{false, true} {
		request, err := photosZoneRequest(auth, shared)
		if err != nil {
			return responses, failure(Configuration, err, nil, nil)
		}

		err = validateOrigin(auth.Origin)
		if err != nil {
			return responses, failure(Configuration, err, nil, nil)
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
			if shared {
				ignored := photosSharedZoneRefusal(err)
				if ignored != nil {
					return append(responses, ignored), nil
				}
			}

			return responses, err
		}

		_, err = decodeReminderZones(response.Body)
		if err != nil {
			if shared {
				return append(responses, response), nil
			}

			return responses, responseFailure(Decode, err, response)
		}

		responses = append(responses, response)
	}

	return responses, nil
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

func photosSharedZoneRefusal(err error) *BytesResponse {
	var failure *ResponseError
	if !errors.As(err, &failure) || failure.Stage != Provider {
		return nil
	}

	return &BytesResponse{Status: failure.Status, Body: failure.Body, Headers: failure.Headers,
		CookieScopeURL: failure.CookieScopeURL}
}
