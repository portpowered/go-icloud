package webtransport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/portpowered/go-icloud/internal/driveapi"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/drive"
)

var errDriveShape = errors.New("drive response is missing its expected records")

// DriveNodeResponse separates a decoded node from owned response metadata.
type DriveNodeResponse struct {
	Data     drive.DriveNode
	Response *BytesResponse
}

// DriveLibrariesResponse separates decoded libraries from owned response metadata.
type DriveLibrariesResponse struct {
	Data     drive.DriveAppLibraries
	Response *BytesResponse
}

// GetDriveNode fetches a fresh node using the caller's explicit Drive origin and sharing selector.
func (client *Client) GetDriveNode(ctx context.Context, auth RequestContext,
	identifier string, share *drive.DriveShareID,
) (*DriveNodeResponse, error) {
	if share != nil && share.Owner == nil && share.Zone == nil && share.Share == nil &&
		len(share.AdditionalProperties) == 0 {
		share = nil
	}

	queries := drive.DriveNodeQueries{{Drivewsid: identifier,
		PartialData: drive.DriveNodeQueryPartialDataFalse, ShareID: share}}

	body, err := referenceJSON(queries)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	params := driveRequestParameters(auth)

	request, err := driveapi.NewDriveRetrieveNodesRequestWithBody(auth.Origin, &params,
		protocol.DriveMediaApplicationJson, bytes.NewReader(body))
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	response, err := client.read(ctx, auth, request, "")
	if err != nil {
		return nil, err
	}

	data, err := decodeDriveNodes(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &DriveNodeResponse{Data: data[0], Response: response}, nil
}

// ListDriveLibraries fetches fresh application-library records for this request's account.
func (client *Client) ListDriveLibraries(ctx context.Context, auth RequestContext) (*DriveLibrariesResponse, error) {
	params := driveapi.DriveListAppLibrariesParams(driveRequestParameters(auth))

	request, err := driveapi.NewDriveListAppLibrariesRequest(auth.Origin, &params)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	response, err := client.read(ctx, auth, request, "")
	if err != nil {
		return nil, err
	}

	data, err := decodeDriveLibraries(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &DriveLibrariesResponse{Data: data, Response: response}, nil
}

func decodeDriveNodes(body []byte) (drive.DriveNodes, error) {
	var data drive.DriveNodes

	var records []json.RawMessage

	err := json.Unmarshal(body, &records)
	if err != nil {
		return nil, fmt.Errorf("decode Drive nodes: %w", err)
	}

	if len(records) == 0 || len(bytes.TrimSpace(records[0])) == 0 || bytes.TrimSpace(records[0])[0] != '{' {
		return nil, errDriveShape
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, fmt.Errorf("decode Drive node fields: %w", err)
	}

	return data, nil
}

func decodeDriveLibraries(body []byte) (drive.DriveAppLibraries, error) {
	var data drive.DriveAppLibraries

	var fields map[string]json.RawMessage

	err := json.Unmarshal(body, &fields)
	if err != nil {
		return data, fmt.Errorf("decode Drive libraries envelope: %w", err)
	}

	items := bytes.TrimSpace(fields[protocol.DriveAppLibrariesItems])
	if len(items) == 0 || items[0] != '[' {
		return data, errDriveShape
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return data, fmt.Errorf("decode Drive libraries: %w", err)
	}

	return data, nil
}

func driveRequestParameters(auth RequestContext) driveapi.DriveRetrieveNodesParams {
	return driveapi.DriveRetrieveNodesParams{ClientId: auth.Params.ClientId, Dsid: auth.Params.Dsid, Token: nil,
		ClientBuildNumber: auth.Params.ClientBuildNumber, ClientMasteringNumber: auth.Params.ClientMasteringNumber,
		Accept: nil, Cookie: nil, Origin: nil, Referer: nil, UserAgent: nil, AcceptEncoding: nil, Connection: nil}
}
