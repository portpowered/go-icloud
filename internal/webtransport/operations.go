package webtransport

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/portpowered/go-icloud/internal/accountapi"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/account"
)

var errAccountShape = errors.New("account response has invalid required fields")

// FamilyResponse retains the decoded family envelope and HTTP metadata.
type FamilyResponse struct {
	Data     account.AccountFamilyResponse
	Metadata *BytesResponse
}

// StorageResponse retains the decoded storage envelope and HTTP metadata.
type StorageResponse struct {
	Data     account.AccountStorageResponse
	Metadata *BytesResponse
}

// GetFamily fetches a fresh family envelope; omission of members is successful.
func (client *Client) GetFamily(ctx context.Context, auth RequestContext) (*FamilyResponse, error) {
	params := new(accountapi.ListAccountFamilyParams)
	params.ClientId, params.Dsid = auth.Params.ClientId, auth.Params.Dsid
	params.Accept = accountapi.ListAccountFamilyParamsAccept(accountapi.AcceptAsterisk)

	request, err := accountapi.NewListAccountFamilyRequest(auth.Origin, params)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	response, err := client.read(ctx, auth, request, "")
	if err != nil {
		return nil, err
	}

	data, err := decodeFamily(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &FamilyResponse{Data: data, Metadata: response}, nil
}

// GetMemberPhoto fetches exact photo bytes without inferring an image encoding.
func (client *Client) GetMemberPhoto(ctx context.Context, auth RequestContext,
	memberID string,
) (*BytesResponse, error) {
	params := new(accountapi.GetFamilyMemberPhotoParams)
	params.ClientId, params.Dsid, params.MemberId = auth.Params.ClientId, auth.Params.Dsid, memberID
	params.Accept = accountapi.GetFamilyMemberPhotoParamsAccept(accountapi.AcceptAsterisk)

	request, err := accountapi.NewGetFamilyMemberPhotoRequest(auth.Origin, params)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	return client.read(ctx, auth, request, "&"+queryPart(protocol.GetFamilyMemberPhotoMemberIdName, memberID))
}

// GetStorage posts an empty entity and fetches fresh storage information.
func (client *Client) GetStorage(ctx context.Context, auth RequestContext) (*StorageResponse, error) {
	params := new(accountapi.GetAccountStorageParams)
	params.ClientId, params.Dsid = auth.Params.ClientId, auth.Params.Dsid
	params.Accept = accountapi.GetAccountStorageParamsAccept(accountapi.AcceptAsterisk)

	request, err := accountapi.NewGetAccountStorageRequest(auth.Origin, params)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	response, err := client.read(ctx, auth, request, "")
	if err != nil {
		return nil, err
	}

	data, err := decodeStorage(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &StorageResponse{Data: data, Metadata: response}, nil
}

// GetPlanSummary returns exact subscription JSON, including unknown fields.
func (client *Client) GetPlanSummary(ctx context.Context, auth RequestContext) (*BytesResponse, error) {
	params := new(accountapi.GetAccountPlanSummaryParams)
	params.ClientId, params.Dsid = auth.Params.ClientId, auth.Params.Dsid
	params.Accept = accountapi.GetAccountPlanSummaryParamsAccept(accountapi.AcceptAsterisk)

	request, err := accountapi.NewGetAccountPlanSummaryRequest(auth.Origin, auth.Params.Dsid, params)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	response, err := client.read(ctx, auth, request, "")
	if err != nil {
		return nil, err
	}

	if !json.Valid(response.Body) {
		return nil, responseFailure(Decode, errAccountShape, response)
	}

	return response, nil
}
