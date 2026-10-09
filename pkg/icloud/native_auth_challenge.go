package icloud

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"

	"github.com/portpowered/go-icloud/internal/authapi"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/bridge"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
	bridgemodels "github.com/portpowered/go-icloud/pkg/dependencymodels/bridge"
)

// GetAuthenticationChallenge retrieves typed MFA choices and a detached bridge bootstrap.
func (sdk *SDK) GetAuthenticationChallenge(ctx context.Context, request NativeAuthRequest) (*NativeAuthResult, error) {
	operation, err := newNativeAuthOperation(ctx, "GetAuthenticationChallenge", request.Auth, request.State)
	if err != nil {
		return nil, err
	}

	err = sdk.nativeGetChallenge(ctx, operation)

	if err != nil {
		return nil, err
	}

	return nativeAuthResult(operation)
}

func (sdk *SDK) nativeGetChallenge(ctx context.Context, operation *nativeAuthOperation) error {
	request, err := authapi.NewGetAuthChallengeRequest(nativeIDMSOrigin(operation.state))
	if err != nil {
		return newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, request, nativeAuthHeaders(operation.state, protocol.AuthHTMLAcceptValue))
	if err != nil {
		return err
	}

	err = nativeRequireSuccess(operation, response)

	if err != nil {
		return err
	}

	challenge := initialNativeAuthState(AuthenticateRequest{}).Challenge

	var data auth.AuthChallenge

	if bytes.HasPrefix(bytes.TrimSpace(response.Body), []byte{'{'}) && json.Unmarshal(response.Body, &data) == nil {
		err = nativeProjectChallenge(&challenge, data)
		if err != nil {
			return nativeResponseError(operation, response, err, InvalidResponse)
		}
	} else {
		bootstrap, parseErr := bridge.ParseBootstrap(response.Body)
		if parseErr != nil {
			return nativeResponseError(operation, response, parseErr, InvalidResponse)
		}

		err = nativeProjectBootstrap(&challenge, bootstrap)

		if err != nil {
			return nativeResponseError(operation, response, err, InvalidResponse)
		}
	}

	operation.state.Challenge = challenge
	operation.state.CodeRequested = false

	operation.state.DeliveryMethod = TwoFactorDeliveryUnknown

	if !nativeHasSecurityKey(challenge) {
		return sdk.nativeProbeSecurityKey(ctx, operation)
	}

	return nil
}

func (sdk *SDK) nativeProbeSecurityKey(ctx context.Context, operation *nativeAuthOperation) error {
	request, err := authapi.NewGetAuthChallengeRequest(nativeIDMSOrigin(operation.state))
	if err != nil {
		return newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, request, nativeAuthHeaders(operation.state, protocol.AuthMediaApplicationJson))
	if err != nil {
		if nativeCanRetry(err) {
			return nil
		}

		return err
	}

	var data auth.AuthChallenge
	if json.Unmarshal(response.Body, &data) == nil {
		nativeProjectSecurityKey(&operation.state.Challenge, data)
	}

	return nil
}

func nativeProjectChallenge(challenge *NativeAuthChallenge, data auth.AuthChallenge) error {
	if data.Mode != nil {
		challenge.Mode = *data.Mode
	}

	if data.AuthInitialRoute != nil {
		challenge.AuthInitialRoute = *data.AuthInitialRoute
	}

	if data.HasTrustedDevices != nil {
		challenge.HasTrustedDevices = *data.HasTrustedDevices
	}

	if data.AuthFactors != nil {
		challenge.AuthFactors = slices.Clone(*data.AuthFactors)
	}

	nativeProjectSecurityKey(challenge, data)

	err := nativeProjectPhones(challenge, data.TrustedPhoneNumber, data.PhoneNumberVerification)

	if err != nil {
		return err
	}

	if data.Direct != nil {
		return nativeProjectBootstrap(challenge, data.Direct)
	}

	if data.BridgeInitiateData != nil {
		bootstrap := bridgemodels.BridgeBootstrapDirect{AuthInitialRoute: data.AuthInitialRoute,
			HasTrustedDevices: data.HasTrustedDevices, TwoSV: &bridgemodels.BridgeBootstrapSecondFactor{
				AuthFactors: data.AuthFactors, BridgeInitiateData: data.BridgeInitiateData, PhoneNumberVerification: nil,
				SourceAppId: nil, AdditionalProperties: nil}, AdditionalProperties: nil}

		return nativeProjectBootstrap(challenge, &bootstrap)
	}

	return nil
}

func nativeProjectSecurityKey(challenge *NativeAuthChallenge, data auth.AuthChallenge) {
	if data.KeyNames != nil {
		challenge.SecurityKeyNames = slices.Clone(*data.KeyNames)
	}

	key := data.FsaChallenge
	if key != nil && key.Challenge != nil && key.KeyHandles != nil && key.RpId != nil {
		challenge.SecurityKeyChallenge = &SecurityKeyChallenge{Challenge: *key.Challenge,
			CredentialIDs: slices.Clone(*key.KeyHandles), RelyingPartyID: *key.RpId}
	}
}

func nativeHasSecurityKey(challenge NativeAuthChallenge) bool {
	return challenge.SecurityKeyChallenge != nil && challenge.SecurityKeyChallenge.Challenge != "" &&
		len(challenge.SecurityKeyChallenge.CredentialIDs) > 0 && challenge.SecurityKeyChallenge.RelyingPartyID != "" &&
		len(challenge.SecurityKeyNames) > 0
}
