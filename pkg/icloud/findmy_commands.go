package icloud

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/findmy"
)

type findMyCapability uint8

const (
	findMySound findMyCapability = iota
	findMyMessage
	findMyLost
	findMyErase
)

type findMyCommand func(context.Context, webtransport.RequestContext) (*webtransport.BytesResponse, error)

// PlaySound requests a sound on a discovered capable device. It never retries a command.
func (session *FindMySession) PlaySound(ctx context.Context, request FindMySoundRequest) (*FindMyCommandResult, error) {
	return session.command(ctx, "FindMyPlaySound", request.DeviceID, findMySound,
		func(call context.Context, auth webtransport.RequestContext) (*webtransport.BytesResponse, error) {
			return session.client.web.PlayFindMySound(call, auth, request.DeviceID,
				findMyText(request.Subject, string(findmy.FindMyIPhoneAlert)))
		})
}

// SendMessage requests a displayed message and selected alert controls on a capable device.
func (session *FindMySession) SendMessage(ctx context.Context,
	request FindMyMessageRequest,
) (*FindMyCommandResult, error) {
	payload := findmy.FindMyMessageRequest{Device: request.DeviceID,
		Subject:  findMyText(request.Subject, string(findmy.FindMyIPhoneAlert)),
		UserText: findmy.FindMyMessageRequestUserTextTrue,
		Text:     findMyText(request.Text, string(findmy.ThisIsANote)),
		Sound:    request.Sound, Vibrate: request.Vibrate, Strobe: request.Strobe}

	return session.command(ctx, "FindMySendMessage", request.DeviceID, findMyMessage,
		func(call context.Context, auth webtransport.RequestContext) (*webtransport.BytesResponse, error) {
			return session.client.web.SendFindMyMessage(call, auth, payload)
		})
}

// MarkLost requests lost mode using the caller's contact number and optional passcode.
func (session *FindMySession) MarkLost(ctx context.Context, request FindMyLostRequest) (*FindMyCommandResult, error) {
	payload := findmy.FindMyLostRequest{Device: request.DeviceID, OwnerNbr: request.PhoneNumber,
		Text:     findMyText(request.Text, string(findmy.ThisDeviceHasBeenLostPleaseCallMe)),
		UserText: findmy.FindMyLostRequestUserTextTrue, LostModeEnabled: findmy.FindMyLostRequestLostModeEnabledTrue,
		TrackingEnabled: findmy.FindMyLostRequestTrackingEnabledTrue, Passcode: request.Passcode}

	return session.command(ctx, "FindMyMarkLost", request.DeviceID, findMyLost,
		func(call context.Context, auth webtransport.RequestContext) (*webtransport.BytesResponse, error) {
			return session.client.web.MarkFindMyLost(call, auth, payload)
		})
}

// Erase requests remote erase after a capability check and a fresh erase-token lookup.
// The result acknowledges the request and does not establish completed physical erasure.
func (session *FindMySession) Erase(ctx context.Context, request FindMyEraseRequest) (*FindMyCommandResult, error) {
	return session.command(ctx, "FindMyErase", request.DeviceID, findMyErase,
		func(call context.Context, auth webtransport.RequestContext) (*webtransport.BytesResponse, error) {
			return session.erase(call, auth, request)
		})
}

func (session *FindMySession) erase(ctx context.Context, auth webtransport.RequestContext,
	request FindMyEraseRequest,
) (*webtransport.BytesResponse, error) {
	credentials := session.Authentication()
	auth.Origin = credentials.SetupServiceURL

	response, err := session.client.web.ObtainFindMyEraseToken(ctx, auth, credentials.SessionToken)
	if err != nil {
		return nil, adaptFailure("FindMyErase", err)
	}

	session.recordResponse(publicMetadata(response.Response))

	if response.Data.Tokens == nil || response.Data.Tokens.MmeFMIPWebEraseDeviceToken == nil {
		return nil, newClientError("FindMyErase", Unavailable, 0, nil, nil, errFindMyEraseToken)
	}

	auth, err = findMyRequestContext(session.Authentication())
	if err != nil {
		return nil, newClientError("FindMyErase", Configuration, 0, nil, nil, err)
	}

	payload := findmy.FindMyEraseRequest{AuthToken: *response.Data.Tokens.MmeFMIPWebEraseDeviceToken,
		Device: request.DeviceID, Text: findMyText(request.Text, string(findmy.ThisDeviceHasBeenLostPleaseCallMe)),
		Passcode: request.Passcode}

	ack, err := session.client.web.EraseFindMyDevice(ctx, auth, payload)
	if err != nil {
		return nil, adaptFailure("FindMyErase", err)
	}

	return ack, nil
}

func (session *FindMySession) command(ctx context.Context, operation, deviceID string,
	capability findMyCapability, send findMyCommand,
) (*FindMyCommandResult, error) {
	call, finish, err := session.begin(ctx, operation)
	if err != nil {
		return nil, err
	}
	defer finish()

	err = session.checkCapability(operation, deviceID, capability)
	if err != nil {
		return nil, err
	}

	boundary, err := findMyRequestContext(session.Authentication())
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	session.clearResponses()

	response, err := send(call, boundary)
	if err != nil {
		return nil, session.recordFailure(operation, adaptFindMyFailure(operation, err))
	}

	session.recordResponse(publicMetadata(response))
	session.clearLastError()

	return &FindMyCommandResult{Responses: session.LastResponses(),
		Acknowledgement: append([]byte(nil), response.Body...)}, nil
}

func (session *FindMySession) checkCapability(operation, id string, capability findMyCapability) error {
	session.mu.Lock()
	device, exists := session.devices[id]
	session.mu.Unlock()

	if !exists {
		return newClientError(operation, NotFound, 0, nil, nil, errFindMyDevice)
	}

	available := false
	if capability == findMyLost {
		available = findMyFlag(device.LostModeCapable)
	} else if device.Features != nil {
		switch capability {
		case findMySound:
			available = findMyFlag(device.Features.SND)
		case findMyMessage:
			available = findMyFlag(device.Features.MSG)
		case findMyErase:
			available = findMyFlag(device.Features.WIP)
		case findMyLost:
		}
	}

	if !available {
		return newClientError(operation, Unavailable, 0, nil, nil, errFindMyCapability)
	}

	return nil
}

func findMyFlag(value *bool) bool { return value != nil && *value }

func findMyText(value *string, fallback string) string {
	if value == nil {
		return fallback
	}

	return *value
}
