package icloud

import (
	"context"
	"encoding/json"

	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/authapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

// ListTrustedDevices enumerates two-step devices without retaining account state on the client.
func (sdk *SDK) ListTrustedDevices(ctx context.Context, request NativeAuthRequest) (*TrustedDevicesResult, error) {
	operation, err := newNativeAuthOperation(ctx, "ListTrustedDevices", request.Auth, request.State)
	if err != nil {
		return nil, err
	}

	params := nativeAuthParams(operation.state)

	wire, err := nativeEncodedRequest(webtransport.ListAuthTrustedDevicesCall{
		Origin: operation.state.Auth.SetupServiceURL, Params: &params,
	})
	if err != nil {
		return nil, newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, wire, requestHeaders(operation.state.Auth.Headers))
	if err != nil {
		return nil, err
	}

	err = nativeRequireSuccess(operation, response)
	if err != nil {
		return nil, err
	}

	var data auth.AuthTrustedDevicesResponse

	err = json.Unmarshal(response.Body, &data)
	if err != nil {
		return nil, nativeResponseError(operation, response, err, InvalidResponse)
	}

	devices, err := nativeProjectDevices(data)
	if err != nil {
		return nil, nativeResponseError(operation, response, err, InvalidResponse)
	}

	return &TrustedDevicesResult{State: cloneNativeAuthState(operation.state), Devices: devices,
		Responses: cloneDriveResponses(operation.responses)}, nil
}

// SendTwoStepCode requests a code using copied caller-owned device metadata.
func (sdk *SDK) SendTwoStepCode(ctx context.Context, request SendTwoStepCodeRequest) (*NativeAuthResult, error) {
	operation, err := newNativeAuthOperation(ctx, "SendTwoStepCode", request.Auth, request.State)
	if err != nil {
		return nil, err
	}

	device, err := nativeDevicePayload(request.Device)
	if err != nil {
		return nil, newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	params := authapi.SendAuthVerificationCodeParams(nativeAuthParams(operation.state))

	wire, err := nativeEncodedRequest(webtransport.SendAuthVerificationCodeCall{
		Origin: operation.state.Auth.SetupServiceURL, Params: &params, Body: device,
	})
	if err != nil {
		return nil, newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, wire, requestHeaders(operation.state.Auth.Headers))
	if err != nil {
		return nil, err
	}

	err = nativeRequireSuccess(operation, response)
	if err != nil {
		return nil, err
	}

	var verdict auth.AuthSuccessResponse

	err = json.Unmarshal(response.Body, &verdict)
	if err != nil {
		return nil, nativeResponseError(operation, response, err, InvalidResponse)
	}

	operation.success = authTrue(verdict.Success)

	return nativeAuthResult(operation)
}

// VerifyTwoStepCode verifies a trusted device's code and obtains session trust.
func (sdk *SDK) VerifyTwoStepCode(ctx context.Context, request VerifyTwoStepCodeRequest) (*NativeAuthResult, error) {
	operation, err := newNativeAuthOperation(ctx, "VerifyTwoStepCode", request.Auth, request.State)
	if err != nil {
		return nil, err
	}

	device, err := nativeDevicePayload(request.Device)
	if err != nil {
		return nil, newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	device.VerificationCode = &request.Code
	trust := true
	device.TrustBrowser = &trust
	params := authapi.ValidateAuthVerificationCodeParams(nativeAuthParams(operation.state))

	wire, err := nativeEncodedRequest(webtransport.ValidateAuthVerificationCodeCall{
		Origin: operation.state.Auth.SetupServiceURL, Params: &params, Body: device,
	})
	if err != nil {
		return nil, newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, wire, requestHeaders(operation.state.Auth.Headers))
	if err != nil {
		return nativeWrongTwoStepResult(operation, err)
	}

	err = nativeRequireSuccess(operation, response)
	if err != nil {
		return nativeWrongTwoStepResult(operation, err)
	}

	err = sdk.nativeTrust(ctx, operation)
	if err != nil {
		return nil, err
	}

	return nativeAuthResult(operation)
}
