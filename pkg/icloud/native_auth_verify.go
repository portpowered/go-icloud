package icloud

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/authapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

// VerifyTwoFactorCode validates a trusted-device or SMS code and refreshes account discovery.
func (sdk *SDK) VerifyTwoFactorCode(ctx context.Context,
	request VerifyTwoFactorCodeRequest,
) (*NativeAuthResult, error) {
	operation, err := newNativeAuthOperation(ctx, "VerifyTwoFactorCode", request.Auth, request.State)
	if err != nil {
		return nil, err
	}

	err = sdk.nativeVerifyCode(ctx, operation, request.Code)
	if err != nil {
		return nil, err
	}

	if operation.success {
		err = sdk.nativeTrust(ctx, operation)
		if err != nil {
			return nil, err
		}
	}

	return nativeAuthResult(operation)
}

func (sdk *SDK) nativeBridgeLegacy(ctx context.Context, state NativeAuthState, code string,
	afterVerification func(),
) (NativeAuthState, *NativeAuthResult, error) {
	operation, err := newNativeAuthOperation(ctx, "VerifyNativeBridgeCode", state.Auth, state)
	if err != nil {
		return cloneNativeAuthState(state), nil, err
	}

	operation.state.DeliveryMethod = TwoFactorDeliveryTrustedDevice
	err = sdk.nativeVerifyCode(ctx, operation, code)

	if afterVerification != nil {
		afterVerification()
	}

	if err != nil {
		return cloneNativeAuthState(operation.state), nil, err
	}

	if operation.success {
		err = sdk.nativeTrust(ctx, operation)
		if err != nil {
			return cloneNativeAuthState(operation.state), nil, err
		}
	}

	result, err := nativeAuthResult(operation)

	return cloneNativeAuthState(operation.state), result, err
}

func (sdk *SDK) nativeVerifyCode(ctx context.Context, operation *nativeAuthOperation, code string) error {
	operation.state.CodeRequested = false
	if code == "" {
		return newClientError(operation.name, Configuration, 0, nil, nil, errNativeAuthInput)
	}

	wire, err := nativeVerificationRequest(operation.state, code)
	if err != nil {
		return newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	accept := protocol.AuthMediaApplicationJson
	if operation.state.DeliveryMethod == TwoFactorDeliverySMS {
		accept = protocol.AuthSMSAcceptValue
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, wire,
		nativeAuthHeaders(operation.state, accept))
	if err != nil {
		if nativeCanRetry(err) {
			operation.success = false

			return nil
		}

		return err
	}

	operation.success = nativeCodeAccepted(operation.state.DeliveryMethod, response)

	return nil
}

func nativeCodeAccepted(delivery TwoFactorDeliveryMethod, response *webtransport.BytesResponse) bool {
	accepted := response.Status >= http.StatusOK && response.Status < http.StatusMultipleChoices

	if delivery != TwoFactorDeliverySMS && response.Status == http.StatusConflict {
		var verdict auth.AuthVerificationResponse

		accepted = json.Unmarshal(response.Body, &verdict) == nil && verdict.SecurityCode != nil &&
			authTrue(verdict.SecurityCode.Valid)
	}

	return accepted
}

func nativeVerificationRequest(state NativeAuthState, code string) (*http.Request, error) {
	securityCode := auth.AuthSecurityCode{Code: code}
	if state.DeliveryMethod != TwoFactorDeliverySMS {
		input := auth.AuthTrustedCodeRequest{SecurityCode: securityCode}

		return nativeEncodedRequest(input, func(body io.Reader) (*http.Request, error) {
			return nativeGeneratedRequest(authapi.NewVerifyAuthTrustedCodeRequestWithBody(nativeIDMSOrigin(state),
				protocol.AuthMediaApplicationJson, body))
		})
	}

	phone, err := nativeSelectedPhone(state.Challenge, nil)
	if err != nil {
		return nil, err
	}

	payload, err := nativePhonePayload(phone)
	if err != nil {
		return nil, err
	}

	mode := phone.PushMode
	if mode == "" {
		mode = string(auth.Sms)
	}

	input := auth.AuthSMSVerificationRequest{PhoneNumber: payload, SecurityCode: securityCode, Mode: mode}

	return nativeEncodedRequest(input, func(body io.Reader) (*http.Request, error) {
		return nativeGeneratedRequest(authapi.NewVerifyAuthSMSRequestWithBody(nativeIDMSOrigin(state),
			protocol.AuthMediaApplicationJson, body))
	})
}
