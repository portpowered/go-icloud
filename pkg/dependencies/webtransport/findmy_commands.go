package webtransport

import (
	"context"
	"io"
	"net/http"

	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/findmyapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/findmy"
)

// PlayFindMySound sends one sound command and never retries an uncertain acknowledgement.
// Capability checks belong to the account-bound device session.
func (client *Client) PlayFindMySound(ctx context.Context, auth RequestContext,
	device, subject string,
) (*BytesResponse, error) {
	params := findmyapi.FindMyPlaySoundParams(findMyParameters(auth))
	payload := findmy.FindMySoundRequest{Device: device, Subject: subject,
		ClientContext: findmy.FindMySoundContext{Fmly: findmy.FindMySoundContextFmlyTrue}}

	return client.postFindMy(ctx, auth, payload, true, func(body io.Reader) (*http.Request, error) {
		return findmyapi.NewFindMyPlaySoundRequestWithBody(auth.Origin, &params,
			jsonMedia(), body)
	})
}

// SendFindMyMessage sends the caller's message and alert controls once.
func (client *Client) SendFindMyMessage(ctx context.Context, auth RequestContext,
	payload findmy.FindMyMessageRequest,
) (*BytesResponse, error) {
	params := findmyapi.FindMySendMessageParams(findMyParameters(auth))

	return client.postFindMy(ctx, auth, payload, true, func(body io.Reader) (*http.Request, error) {
		return findmyapi.NewFindMySendMessageRequestWithBody(auth.Origin, &params,
			jsonMedia(), body)
	})
}

// MarkFindMyLost sends the caller's lost-device message and contact number once.
func (client *Client) MarkFindMyLost(ctx context.Context, auth RequestContext,
	payload findmy.FindMyLostRequest,
) (*BytesResponse, error) {
	params := findmyapi.FindMyLostDeviceParams(findMyParameters(auth))

	return client.postFindMy(ctx, auth, payload, true, func(body io.Reader) (*http.Request, error) {
		return findmyapi.NewFindMyLostDeviceRequestWithBody(auth.Origin, &params,
			jsonMedia(), body)
	})
}

// EraseFindMyDevice sends an already authorized erase command once.
// Token lookup and the command use distinct origins and account query behavior.
func (client *Client) EraseFindMyDevice(ctx context.Context, auth RequestContext,
	payload findmy.FindMyEraseRequest,
) (*BytesResponse, error) {
	params := findmyapi.FindMyEraseDeviceParams(findMyParameters(auth))

	return client.postFindMy(ctx, auth, payload, true, func(body io.Reader) (*http.Request, error) {
		return findmyapi.NewFindMyEraseDeviceRequestWithBody(auth.Origin, &params,
			jsonMedia(), body)
	})
}
