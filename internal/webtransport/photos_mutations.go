package webtransport

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/portpowered/go-icloud/internal/photosapi"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/photosmutations"
)

// ModifyPhotos sends one generated album or asset mutation and preserves provider evidence.
func (client *Client) ModifyPhotos(ctx context.Context, auth RequestContext,
	input photosmutations.PhotoMutationRequest,
) (*ReminderModificationResponse, error) {
	body, err := referenceJSON(input)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	request, err := photosModificationRequest(auth, body)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	response, err := client.readPhotosContainerRequest(ctx, auth, request)
	if err != nil {
		return nil, err
	}

	data, err := decodeReminderModification(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, fmt.Errorf("decode photo modification: %w", err), response)
	}

	return &ReminderModificationResponse{Data: data, Metadata: response}, nil
}
func photosModificationRequest(auth RequestContext, body []byte) (*http.Request, error) {
	if auth.PhotoShared {
		params := photoSharedModificationParams(auth)

		request, err := photosapi.NewPhotosSharedModifyRecordsRequestWithBody(auth.Origin, params,
			protocol.PhotosMediaApplicationJson, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("construct shared photo modification: %w", err)
		}

		return request, nil
	}

	params := photoPrivateModificationParams(auth)

	request, err := photosapi.NewPhotosModifyRecordsRequestWithBody(auth.Origin, params,
		protocol.PhotosMediaApplicationJson, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("construct private photo modification: %w", err)
	}

	return request, nil
}

func photoSharedModificationParams(auth RequestContext) *photosapi.PhotosSharedModifyRecordsParams {
	params := new(photosapi.PhotosSharedModifyRecordsParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = photosapi.PhotosSharedModifyRecordsParamsRemapEnumsTrue
	params.GetCurrentSyncToken = photosapi.PhotosSharedModifyRecordsParamsGetCurrentSyncTokenTrue

	return params
}
func photoPrivateModificationParams(auth RequestContext) *photosapi.PhotosModifyRecordsParams {
	params := new(photosapi.PhotosModifyRecordsParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = photosapi.PhotosModifyRecordsParamsRemapEnumsTrue
	params.GetCurrentSyncToken = photosapi.PhotosModifyRecordsParamsGetCurrentSyncTokenTrue

	return params
}
