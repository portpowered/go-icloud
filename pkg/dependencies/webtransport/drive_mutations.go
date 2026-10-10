package webtransport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/driveapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/drive"
)

// DriveSelection holds the identifier and concurrency token needed by a mutation.
type DriveSelection struct {
	NodeID string
	ETag   string
}

// DriveChangedResponse owns item acknowledgements and their response metadata.
type DriveChangedResponse struct {
	Data     drive.DriveChangedItems
	Response *BytesResponse
}

// DriveCreatedResponse owns created-folder records and their response metadata.
type DriveCreatedResponse struct {
	Data     drive.DriveCreatedFolders
	Response *BytesResponse
}

type driveRequestBuilder func(io.Reader) (*http.Request, error)

// CreateDriveFolder creates one folder with a fresh reference-compatible temporary identifier.
func (client *Client) CreateDriveFolder(ctx context.Context, auth RequestContext,
	parent, name string,
) (*DriveCreatedResponse, error) {
	identifier, err := temporaryFolderID()
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	params := driveapi.DriveCreateFoldersParams(driveRequestParameters(auth))
	payload := drive.DriveCreateFolders{DestinationDrivewsId: parent,
		Folders: []drive.DriveFolderCreation{{ClientId: identifier, Name: name}}}

	auth.Headers = auth.Headers.Clone()
	auth.Headers.Set(protocol.HTTPContentTypeName, protocol.DriveMediaPlainText)

	response, err := client.postDrive(ctx, auth, payload,
		func(body io.Reader) (*http.Request, error) {
			return driveapi.NewDriveCreateFoldersRequestWithBody(auth.Origin, &params,
				protocol.DriveMediaPlainText, body)
		})
	if err != nil {
		return nil, err
	}

	var data drive.DriveCreatedFolders

	err = decodeDriveObject(response.Body, &data)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &DriveCreatedResponse{Data: data, Response: response}, nil
}

// RenameDriveNode requests the desired name without refreshing or retrying the version token.
func (client *Client) RenameDriveNode(ctx context.Context, auth RequestContext,
	node DriveSelection, name string,
) (*DriveChangedResponse, error) {
	params := driveapi.DriveRenameItemsParams(driveRequestParameters(auth))
	payload := drive.DriveRenameItems{Items: []drive.DriveRenameItem{
		{Drivewsid: node.NodeID, Etag: node.ETag, Name: name},
	}}

	return client.changeDriveItems(ctx, auth, payload, func(body io.Reader) (*http.Request, error) {
		return driveapi.NewDriveRenameItemsRequestWithBody(auth.Origin, &params,
			protocol.DriveMediaApplicationJson, body)
	})
}

// MoveDriveNodes preserves node order and permits an empty selection.
func (client *Client) MoveDriveNodes(ctx context.Context, auth RequestContext,
	nodes []DriveSelection, destination string,
) (*DriveChangedResponse, error) {
	items := make([]drive.DriveMoveItem, 0, len(nodes))
	for _, node := range nodes {
		items = append(items, drive.DriveMoveItem{Drivewsid: node.NodeID, Etag: node.ETag, ClientId: node.NodeID})
	}

	params := driveapi.DriveMoveItemsParams(driveRequestParameters(auth))
	payload := drive.DriveMoveItems{DestinationDrivewsId: destination, Items: items}

	return client.changeDriveItems(ctx, auth, payload, func(body io.Reader) (*http.Request, error) {
		return driveapi.NewDriveMoveItemsRequestWithBody(auth.Origin, &params, protocol.DriveMediaApplicationJson, body)
	})
}

// TrashDriveNode uses the node identifier as the source's mutation client identifier.
func (client *Client) TrashDriveNode(ctx context.Context, auth RequestContext,
	node DriveSelection,
) (*DriveChangedResponse, error) {
	params := driveapi.DriveTrashItemsParams(driveRequestParameters(auth))
	payload := drive.DriveTrashItems{Items: []drive.DriveMoveItem{
		{Drivewsid: node.NodeID, Etag: node.ETag, ClientId: node.NodeID},
	}}

	return client.changeDriveItems(ctx, auth, payload, func(body io.Reader) (*http.Request, error) {
		return driveapi.NewDriveTrashItemsRequestWithBody(auth.Origin, &params, protocol.DriveMediaApplicationJson, body)
	})
}

// RestoreDriveNode restores the selected trash item without adding a client identifier.
func (client *Client) RestoreDriveNode(ctx context.Context, auth RequestContext,
	node DriveSelection,
) (*DriveChangedResponse, error) {
	params := driveapi.DriveRecoverItemsParams(driveRequestParameters(auth))
	payload := drive.DriveRecoverItems{Items: []drive.DriveRecoveryItem{{Drivewsid: node.NodeID, Etag: node.ETag}}}

	return client.changeDriveItems(ctx, auth, payload, func(body io.Reader) (*http.Request, error) {
		return driveapi.NewDriveRecoverItemsRequestWithBody(auth.Origin, &params, protocol.DriveMediaApplicationJson, body)
	})
}

// DeleteDriveNode includes the account's authenticated client identifier.
func (client *Client) DeleteDriveNode(ctx context.Context, auth RequestContext,
	node DriveSelection,
) (*DriveChangedResponse, error) {
	return client.deleteDriveNode(ctx, auth, node, &auth.Params.ClientId)
}

// PermanentlyDeleteDriveNode omits the client identifier for permanent trash deletion.
func (client *Client) PermanentlyDeleteDriveNode(ctx context.Context, auth RequestContext,
	node DriveSelection,
) (*DriveChangedResponse, error) {
	return client.deleteDriveNode(ctx, auth, node, nil)
}

func (client *Client) deleteDriveNode(ctx context.Context, auth RequestContext,
	node DriveSelection, clientID *string,
) (*DriveChangedResponse, error) {
	params := driveapi.DriveDeleteItemsParams(driveRequestParameters(auth))
	payload := drive.DriveItemChanges{Items: []drive.DriveItemChange{
		{Drivewsid: node.NodeID, Etag: node.ETag, ClientId: clientID},
	}}

	return client.changeDriveItems(ctx, auth, payload, func(body io.Reader) (*http.Request, error) {
		return driveapi.NewDriveDeleteItemsRequestWithBody(auth.Origin, &params, protocol.DriveMediaApplicationJson, body)
	})
}

func (client *Client) postDrive(ctx context.Context, auth RequestContext,
	payload any, build driveRequestBuilder,
) (*BytesResponse, error) {
	body, err := referenceJSON(payload)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	request, err := build(bytes.NewReader(body))
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	return client.read(ctx, auth, request, "")
}

func (client *Client) changeDriveItems(ctx context.Context, auth RequestContext,
	payload any, build driveRequestBuilder,
) (*DriveChangedResponse, error) {
	response, err := client.postDrive(ctx, auth, payload, build)
	if err != nil {
		return nil, err
	}

	var data drive.DriveChangedItems

	err = decodeDriveObject(response.Body, &data)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &DriveChangedResponse{Data: data, Response: response}, nil
}

func decodeDriveObject(body []byte, target any) error {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return errDriveShape
	}

	err := json.Unmarshal(body, target)
	if err != nil {
		return fmt.Errorf("decode Drive acknowledgement: %w", err)
	}

	return nil
}
