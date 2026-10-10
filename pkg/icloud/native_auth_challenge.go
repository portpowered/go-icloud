package icloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/bridge"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/authapi"
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

	response, err := sdk.nativeAuthExchange(ctx, operation, request,
		nativeAuthHeaders(operation.state, protocol.AuthHTMLAcceptValue))
	if err != nil {
		return err
	}

	err = nativeRequireSuccess(operation, response)
	if err != nil {
		return err
	}

	challenge, err := nativeResponseChallenge(response.Body)
	if err != nil {
		return nativeResponseError(operation, response, err, InvalidResponse)
	}

	operation.state.Challenge = challenge
	operation.state.CodeRequested = false

	operation.state.DeliveryMethod = TwoFactorDeliveryUnknown
	operation.state.DeliveryNotice = nil

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

	response, err := sdk.nativeAuthExchange(ctx, operation, request,
		nativeAuthHeaders(operation.state, protocol.AuthMediaApplicationJson))
	if err != nil {
		if nativeCanRetry(err) {
			return nil
		}

		return err
	}

	var data auth.AuthChallenge
	if !nativeOptionalChallenge(response.Body, &data) {
		return nil
	}

	nativeProjectSecurityKey(&operation.state.Challenge, data)

	err = nativeRetainSecurityChallenge(&operation.state.Challenge, data)
	if err != nil {
		return nativeResponseError(operation, response, err, InvalidResponse)
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
		second, err := nativeSecondFactor(data)
		if err != nil {
			return err
		}

		bootstrap := bridgemodels.BridgeBootstrapDirect{AuthInitialRoute: data.AuthInitialRoute,
			HasTrustedDevices: data.HasTrustedDevices, TwoSV: second, AdditionalProperties: nil}

		return nativeProjectBootstrap(challenge, &bootstrap)
	}

	return nil
}

func nativeSecondFactor(data auth.AuthChallenge) (*bridgemodels.BridgeBootstrapSecondFactor, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("encode native second factor: %w", err)
	}

	var second bridgemodels.BridgeBootstrapSecondFactor

	err = json.Unmarshal(encoded, &second)
	if err != nil {
		return nil, fmt.Errorf("decode native second factor: %w", err)
	}

	return &second, nil
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

func nativeResponseChallenge(body []byte) (NativeAuthChallenge, error) {
	data, fromHTML, err := nativeDecodeChallenge(body)
	if err != nil {
		return emptyNativeAuthChallenge(), err
	}

	data, err = nativeNormalizeChallenge(data)
	if err != nil {
		return emptyNativeAuthChallenge(), err
	}

	challenge := emptyNativeAuthChallenge()

	err = nativeProjectChallenge(&challenge, data)
	if err != nil {
		return emptyNativeAuthChallenge(), err
	}

	if fromHTML {
		data.Direct = nil
	}

	challenge.ProviderData, err = json.Marshal(data)
	if err != nil {
		return emptyNativeAuthChallenge(), fmt.Errorf("encode native challenge: %w", err)
	}

	return challenge, nil
}

func nativeDecodeChallenge(body []byte) (auth.AuthChallenge, bool, error) {
	data := new(auth.AuthChallenge)
	if bytes.HasPrefix(bytes.TrimSpace(body), []byte{'{'}) && json.Unmarshal(body, data) == nil {
		return *data, false, nil
	}

	bootstrap, err := bridge.ParseBootstrap(body)
	if err != nil {
		return *data, false, fmt.Errorf("parse native challenge bootstrap: %w", err)
	}

	data = new(auth.AuthChallenge)
	data.Direct = bootstrap

	return *data, true, nil
}

func nativeRetainSecurityChallenge(challenge *NativeAuthChallenge, data auth.AuthChallenge) error {
	var retained auth.AuthChallenge
	if !nativeOptionalChallenge(challenge.ProviderData, &retained) {
		return nil
	}

	if data.FsaChallenge != nil {
		retained.FsaChallenge = data.FsaChallenge
	}

	if data.KeyNames != nil {
		retained.KeyNames = data.KeyNames
	}

	encoded, err := json.Marshal(retained)
	if err != nil {
		return fmt.Errorf("encode native security challenge: %w", err)
	}

	challenge.ProviderData = encoded

	return nil
}

func nativeOptionalChallenge(body []byte, data *auth.AuthChallenge) bool {
	return json.Unmarshal(body, data) == nil
}
