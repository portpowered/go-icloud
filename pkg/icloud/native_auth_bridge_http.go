package icloud

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
	bridgemodels "github.com/portpowered/go-icloud/pkg/dependencymodels/bridge"
)

func (sdk *SDK) nativeBridgeExchange(ctx context.Context, state NativeAuthState, input bridgemodels.BridgeExchange,
) (NativeAuthState, ResponseMetadata, error) {
	if input.NextStep == bridgemodels.BootstrapStep {
		updated, metadata, _, err := sdk.nativeBridgeStart(ctx, state, auth.AuthBridgeStartRequest{
			SessionUUID: input.SessionUUID, Ptkn: input.Ptkn})

		return updated, metadata, err
	}

	data := []byte{}

	var err error

	if input.Data != nil {
		data, err = base64.StdEncoding.Strict().DecodeString(*input.Data)
	}

	if err != nil {
		return state, ResponseMetadata{}, newClientError("OpenNativeBridgeSession", Configuration, 0, nil, nil, err)
	}

	akdata, err := webtransport.EncodeBridgeOpaqueData(input.Akdata)
	if err != nil {
		return state, ResponseMetadata{}, newClientError("OpenNativeBridgeSession", Configuration, 0, nil, nil, err)
	}

	request := auth.AuthBridgeStepRequest{SessionUUID: input.SessionUUID, Data: data, Ptkn: input.Ptkn,
		NextStep: auth.AuthBridgeStepRequestNextStep(input.NextStep), Idmsdata: input.Idmsdata, Akdata: akdata}
	updated, metadata, _, err := sdk.nativeBridgeStep(ctx, state, int(input.NextStep), request)

	return updated, metadata, err
}

func (sdk *SDK) nativeBridgeStart(ctx context.Context, state NativeAuthState, input auth.AuthBridgeStartRequest,
) (NativeAuthState, ResponseMetadata, *auth.AuthBridgeResponse, error) {
	request, err := nativeEncodedRequest(webtransport.AuthBridgeStep0Call{
		Origin: nativeIDMSOrigin(state), Params: nil, Body: input})
	if err != nil {
		return state, ResponseMetadata{}, nil, newClientError("OpenNativeBridgeSession", Configuration, 0, nil, nil, err)
	}

	return sdk.nativeBridgeHTTP(ctx, state, request)
}

func (sdk *SDK) nativeBridgeStep(ctx context.Context, state NativeAuthState, step int, input auth.AuthBridgeStepRequest,
) (NativeAuthState, ResponseMetadata, *auth.AuthBridgeResponse, error) {
	request, err := nativeBridgeStepRequest(state, step, input)
	if err != nil {
		return state, ResponseMetadata{}, nil, newClientError("OpenNativeBridgeSession", Configuration, 0, nil, nil, err)
	}

	return sdk.nativeBridgeHTTP(ctx, state, request)
}

//nolint:ireturn // GO-15: selects the sealed schema-owned step 2, 4 or 6 operation without an untyped body.
func nativeBridgeStepRequest(state NativeAuthState, step int,
	body auth.AuthBridgeStepRequest,
) (webtransport.AuthenticationCall, error) {
	switch step {
	case int(auth.N2):
		return nativeEncodedRequest(webtransport.AuthBridgeStep2Call{
			Origin: nativeIDMSOrigin(state), Params: nil, Body: body,
		})
	case int(auth.N4):
		return nativeEncodedRequest(webtransport.AuthBridgeStep4Call{
			Origin: nativeIDMSOrigin(state), Params: nil, Body: body,
		})
	case int(auth.N6):
		return nativeEncodedRequest(webtransport.AuthBridgeStep6Call{
			Origin: nativeIDMSOrigin(state), Params: nil, Body: body,
		})
	default:
		return nil, errNativeAuthInput
	}
}

func (sdk *SDK) nativeBridgeHTTP(ctx context.Context, state NativeAuthState, request webtransport.AuthenticationCall,
) (NativeAuthState, ResponseMetadata, *auth.AuthBridgeResponse, error) {
	operation, err := newNativeAuthOperation(ctx, "OpenNativeBridgeSession", state.Auth, state)
	if err != nil {
		return state, ResponseMetadata{}, nil, err
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, request, nativeBridgeHeaders(state))
	if err != nil {
		return operation.state, nativeBridgeFailureMetadata(operation), nil, err
	}

	if response.Status != http.StatusOK && response.Status != http.StatusNoContent &&
		response.Status != http.StatusConflict {
		return operation.state, publicMetadata(response), nil,
			nativeResponseError(operation, response, errNativeAuthInput, providerKind(response.Status))
	}

	var data auth.AuthBridgeResponse
	if len(response.Body) > 0 {
		_ = json.Unmarshal(response.Body, &data)
	}

	return operation.state, publicMetadata(response), &data, nil
}

func (sdk *SDK) nativeBridgeCode(ctx context.Context, state NativeAuthState, input auth.AuthBridgeCodeRequest,
) (NativeAuthState, ResponseMetadata, int, error) {
	operation, err := newNativeAuthOperation(ctx, "VerifyNativeBridgeCode", state.Auth, state)
	if err != nil {
		return state, ResponseMetadata{}, 0, err
	}

	request, err := nativeEncodedRequest(webtransport.ValidateAuthBridgeCodeCall{
		Origin: nativeIDMSOrigin(state), Params: nil, Body: input})
	if err != nil {
		return state, ResponseMetadata{}, 0, newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, request, nativeBridgeHeaders(state))
	if err != nil {
		return operation.state, nativeBridgeFailureMetadata(operation), 0, err
	}

	if response.Status != http.StatusOK && response.Status != http.StatusNoContent &&
		response.Status != http.StatusConflict && response.Status != http.StatusPreconditionFailed {
		return operation.state, publicMetadata(response), response.Status,
			nativeResponseError(operation, response, errNativeAuthInput, providerKind(response.Status))
	}

	return operation.state, publicMetadata(response), response.Status, nil
}

func nativeBridgeFailureMetadata(operation *nativeAuthOperation) ResponseMetadata {
	if operation.response == nil {
		return ResponseMetadata{CookieScopeURL: "", Headers: nil, StatusCode: 0}
	}

	return publicMetadata(operation.response)
}

func nativeBridgeHeaders(state NativeAuthState) http.Header {
	headers := nativeAuthHeaders(state, nativeJSONMedia())

	var bootstrap bridgemodels.BridgeBootstrapDirect

	if json.Unmarshal(state.Challenge.BridgeBootstrap, &bootstrap) != nil || bootstrap.TwoSV == nil ||
		bootstrap.TwoSV.SourceAppId == nil {
		return headers
	}

	value, err := bootstrap.TwoSV.SourceAppId.AsBridgeBootstrapSecondFactorSourceAppId0()
	if err != nil {
		number, decodeErr := bootstrap.TwoSV.SourceAppId.AsBridgeBootstrapSecondFactorSourceAppId1()
		if decodeErr != nil {
			return headers
		}

		value = strconv.Itoa(number)
	}

	if value != "" {
		headers.Set(protocol.AuthHTTPXAppleAppIdName, value)
	}

	return headers
}
