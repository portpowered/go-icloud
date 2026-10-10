package webtransport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/findmyapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/findmy"
)

// FindMyDevicesResponse retains the partial discovery envelope and owned HTTP evidence.
type FindMyDevicesResponse struct {
	Data     findmy.FindMyRefreshResponse
	Response *BytesResponse
	// Context preserves the provider's object member order for the next refresh.
	Context findmy.FindMyRefreshContext
}

// FindMyTokenResponse retains token lookup fields and owned HTTP evidence.
type FindMyTokenResponse struct {
	Data     findmy.FindMyEraseTokenResponse
	Response *BytesResponse
}

type findMyRequestBuilder func(io.Reader) (*http.Request, error)

// InitializeFindMy discovers devices without a previous server context.
// The initial request deliberately omits refresh-only location controls.
func (client *Client) InitializeFindMy(ctx context.Context, auth RequestContext,
	family bool,
) (*FindMyDevicesResponse, error) {
	params := findMyParameters(auth)
	payload := findmy.FindMyInitializeRequest{ClientContext: findmy.FindMyInitializeContext{
		AppName: findmy.ICloudFindWeb, AppVersion: findmy.N20, ApiVersion: findmy.N30,
		DeviceListVersion: findmy.FindMyInitializeContextDeviceListVersionN1, Fmly: family,
		Timezone: findmy.USPacific, InactiveTime: findmy.FindMyInitializeContextInactiveTimeN0,
	}}

	response, err := client.postFindMy(ctx, auth, payload, true, func(body io.Reader) (*http.Request, error) {
		return findmyapi.NewFindMyInitializeRequestWithBody(auth.Origin, &params,
			jsonMedia(), body)
	})

	return decodeFindMyDevices(response, err)
}

// RefreshFindMy uses the caller's previous provider context without retaining account state.
// The session layer decides whether an absent/empty context requires initialization instead.
func (client *Client) RefreshFindMy(ctx context.Context, auth RequestContext,
	server findmy.FindMyRefreshContext, family, locate bool,
) (*FindMyDevicesResponse, error) {
	server, err := clearFindMyTheftLoss(server)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	params := findmyapi.FindMyRefreshDevicesParams(findMyParameters(auth))
	payload := findmy.FindMyRefreshRequest{ClientContext: findmy.FindMyClientContext{
		AppName: findmy.ICloudFindWeb, AppVersion: findmy.N20, ApiVersion: findmy.N30,
		DeviceListVersion: findmy.FindMyClientContextDeviceListVersionN1, Fmly: family,
		Timezone: findmy.USPacific, InactiveTime: findmy.FindMyClientContextInactiveTimeN0,
		ShouldLocate: nil, SelectedDevice: nil,
	}, ServerContext: nullable.NewNullableWithValue(server), IsUpdatingAllLocations: nil}

	if locate {
		shouldLocate := findmy.FindMyClientContextShouldLocateTrue
		selected := findmy.All
		updating := findmy.FindMyRefreshRequestIsUpdatingAllLocationsTrue
		payload.ClientContext.ShouldLocate, payload.ClientContext.SelectedDevice = &shouldLocate, &selected
		payload.IsUpdatingAllLocations = &updating
	}

	response, err := client.postFindMy(ctx, auth, payload, true, func(body io.Reader) (*http.Request, error) {
		return findmyapi.NewFindMyRefreshDevicesRequestWithBody(auth.Origin, &params,
			jsonMedia(), body)
	})

	return decodeFindMyDevices(response, err)
}

// ObtainFindMyEraseToken uses the setup origin and sends no account query parameters.
// The session layer must verify capability and token presence before issuing an erase.
func (client *Client) ObtainFindMyEraseToken(ctx context.Context, auth RequestContext,
	token *string,
) (*FindMyTokenResponse, error) {
	payload := findmy.FindMyEraseTokenRequest{DsWebAuthToken: nullable.NewNullNullable[string]()}
	if token != nil {
		payload.DsWebAuthToken = nullable.NewNullableWithValue(*token)
	}

	params := new(findmyapi.FindMyObtainEraseTokenParams)

	response, err := client.postFindMy(ctx, auth, payload, false, func(body io.Reader) (*http.Request, error) {
		return findmyapi.NewFindMyObtainEraseTokenRequestWithBody(auth.Origin, params,
			jsonMedia(), body)
	})
	if err != nil {
		return nil, err
	}

	var data findmy.FindMyEraseTokenResponse

	err = decodeFindMyObject(response.Body, &data)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &FindMyTokenResponse{Data: data, Response: response}, nil
}

func decodeFindMyDevices(response *BytesResponse, err error) (*FindMyDevicesResponse, error) {
	if err != nil {
		return nil, err
	}

	var data findmy.FindMyRefreshResponse

	err = decodeFindMyObject(response.Body, &data)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	fields, err := accountFields(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &FindMyDevicesResponse{Data: data, Response: response,
		Context: fields[protocol.FindMyRefreshResponseServerContext]}, nil
}

func decodeFindMyObject(body []byte, destination any) error {
	_, err := accountFields(body)
	if err != nil {
		return fmt.Errorf("decode Find My envelope: %w", err)
	}

	err = json.Unmarshal(body, destination)
	if err != nil {
		return fmt.Errorf("decode Find My fields: %w", err)
	}

	return nil
}

func findMyParameters(auth RequestContext) findmyapi.FindMyInitializeParams {
	return findmyapi.FindMyInitializeParams{ClientId: auth.Params.ClientId, Dsid: auth.Params.Dsid,
		ClientBuildNumber: auth.Params.ClientBuildNumber, ClientMasteringNumber: auth.Params.ClientMasteringNumber,
		Accept: nil, Cookie: nil, Origin: nil, Referer: nil, UserAgent: nil, AcceptEncoding: nil, Connection: nil}
}
