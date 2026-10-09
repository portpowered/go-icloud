package webtransport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/portpowered/go-icloud/internal/photosapi"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// PhotosDatabaseResponse retains changed-zone identities and complete response evidence.
type PhotosDatabaseResponse struct {
	Data     cloudkit.CKDatabaseChangesResponse
	Metadata *BytesResponse
}

// PhotosDatabaseChanges reads one database-level page, without initializing individual libraries.
func (client *Client) PhotosDatabaseChanges(
	ctx context.Context,
	auth RequestContext,
	since *string,
) (*PhotosDatabaseResponse, error) {
	input := new(cloudkit.CKDatabaseChangesRequest)
	if since != nil && *since != "" {
		input.SyncToken = since
	}

	body, err := referenceJSON(input)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	request, err := photosDatabaseRequest(auth, body)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	response, err := client.readPhotosContainerRequest(ctx, auth, request)
	if err != nil {
		return nil, err
	}

	data, err := decodePhotosDatabase(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &PhotosDatabaseResponse{Data: data, Metadata: response}, nil
}
func photosDatabaseRequest(auth RequestContext, body []byte) (*http.Request, error) {
	if auth.PhotoShared {
		params := photosSharedDatabaseChangesParams(auth)
		request, err := photosapi.NewPhotosSharedDatabaseChangesRequestWithBody(
			auth.Origin,
			params,
			protocol.PhotosMediaApplicationJson,
			bytes.NewReader(body),
		)
		if err != nil {
			return nil, fmt.Errorf("construct shared photo database changes: %w", err)
		}

		return request, nil
	}

	params := photosDatabaseChangesParams(auth)
	request, err := photosapi.NewPhotosDatabaseChangesRequestWithBody(
		auth.Origin,
		params,
		protocol.PhotosMediaApplicationJson,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("construct photo database changes: %w", err)
	}

	return request, nil
}
func decodePhotosDatabase(body []byte) (cloudkit.CKDatabaseChangesResponse, error) {
	var data cloudkit.CKDatabaseChangesResponse

	_, err := decodeReminderZones(body)
	if err != nil {
		return data, err
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return data, fmt.Errorf("decode photo database changes: %w", err)
	}

	return data, nil
}

func photosSharedDatabaseChangesParams(auth RequestContext) *photosapi.PhotosSharedDatabaseChangesParams {
	params := new(photosapi.PhotosSharedDatabaseChangesParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = photosapi.PhotosSharedDatabaseChangesParamsRemapEnumsTrue
	params.GetCurrentSyncToken = photosapi.PhotosSharedDatabaseChangesParamsGetCurrentSyncTokenTrue

	return params
}

func photosDatabaseChangesParams(auth RequestContext) *photosapi.PhotosDatabaseChangesParams {
	params := new(photosapi.PhotosDatabaseChangesParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = photosapi.PhotosDatabaseChangesParamsRemapEnumsTrue
	params.GetCurrentSyncToken = photosapi.PhotosDatabaseChangesParamsGetCurrentSyncTokenTrue

	return params
}
