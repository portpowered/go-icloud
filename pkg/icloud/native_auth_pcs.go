package icloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/authapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

const (
	nativePCSAttempts = 10
	nativePCSInterval = 5 * time.Second
)

// RequestPCSAccess requests protected service access and polls Apple's consent and cookie readiness.
func (sdk *SDK) RequestPCSAccess(ctx context.Context, input RequestPCSAccessRequest) (*NativeAuthResult, error) {
	operation, err := newNativeAuthOperation(ctx, "RequestPCSAccess", input.Auth, input.State)
	if err != nil {
		return nil, err
	}

	if input.Service == "" {
		return nil, newClientError(operation.name, Configuration, 0, nil, nil, errNativeAuthInput)
	}

	status, err := sdk.nativePCSConsent(ctx, operation, false)
	if err != nil {
		return nil, err
	}

	if !authTrue(status.IsICDRSDisabled) {
		return nativeAuthResult(operation)
	}

	if !nativePCSConsented(status) {
		notification, notifyErr := sdk.nativePCSConsent(ctx, operation, true)
		if notifyErr != nil {
			return nil, notifyErr
		}

		if !authTrue(notification.IsDeviceConsentNotificationSent) {
			return nil, nativeResponseError(operation, operation.response, errNativeAuthInput, Provider)
		}
	}

	err = sdk.nativeAwaitPCSConsent(ctx, operation, status)
	if err != nil {
		return nil, err
	}

	err = sdk.nativeAwaitPCSCookies(ctx, operation, input.Service)
	if err != nil {
		return nil, err
	}

	return nativeAuthResult(operation)
}

func nativePCSConsented(status *auth.AuthWebAccessResponse) bool {
	if !status.IsDeviceConsentedForPCS.IsSpecified() {
		return true
	}

	consented, err := status.IsDeviceConsentedForPCS.Get()
	return err == nil && consented
}

func (sdk *SDK) nativePCSConsent(ctx context.Context, operation *nativeAuthOperation,
	notify bool,
) (*auth.AuthWebAccessResponse, error) {
	params := nativeAuthParams(operation.state)

	var (
		request *http.Request
		err     error
	)

	if notify {
		converted := authapi.EnableAuthPCSConsentParams(params)
		request, err = authapi.NewEnableAuthPCSConsentRequest(operation.state.Auth.SetupServiceURL, &converted)
	} else {
		converted := authapi.GetAuthWebAccessStateParams(params)
		request, err = authapi.NewGetAuthWebAccessStateRequest(operation.state.Auth.SetupServiceURL, &converted)
	}

	if err != nil {
		return nil, newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, request, requestHeaders(operation.state.Auth.Headers))
	if err != nil {
		return nil, err
	}

	err = nativeRequireSuccess(operation, response)
	if err != nil {
		return nil, err
	}

	var result auth.AuthWebAccessResponse

	err = nativeDecodeAuthObject(response.Body, &result)
	if err != nil {
		return nil, nativeResponseError(operation, response, err, InvalidResponse)
	}

	return &result, nil
}

func nativeDecodeAuthObject(body []byte, target any) error {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return errNativeAuthInput
	}

	err := json.Unmarshal(body, target)
	if err != nil {
		return fmt.Errorf("decode authentication object: %w", err)
	}

	return nil
}

func (sdk *SDK) nativeAwaitPCSConsent(ctx context.Context, operation *nativeAuthOperation,
	status *auth.AuthWebAccessResponse,
) error {
	for range nativePCSAttempts {
		if nativePCSConsented(status) {
			return nil
		}

		err := sdk.nativePCSWait(ctx, operation)
		if err != nil {
			return err
		}

		refreshed, err := sdk.nativePCSConsent(ctx, operation, false)
		if err != nil {
			return err
		}

		status = refreshed
	}

	return nil
}

func (sdk *SDK) nativePCSWait(ctx context.Context, operation *nativeAuthOperation) error {
	err := sdk.authWait(ctx, nativePCSInterval)
	if err == nil {
		return nil
	}

	failure := driveContextFailure(operation.name, err)
	failure.prior = cloneDriveResponses(operation.responses)

	return failure
}

func (sdk *SDK) nativeAwaitPCSCookies(ctx context.Context, operation *nativeAuthOperation, service string) error {
	for attempt := range nativePCSAttempts {
		status, err := sdk.nativePCSRequest(ctx, operation, service, attempt == 0)
		if err != nil {
			return err
		}

		if status.Status != nil && *status.Status == protocol.AuthPCSSuccessStatusValue {
			return nil
		}

		if status.Status == nil || status.Message == nil {
			return nativeResponseError(operation, operation.response, errNativeAuthInput, InvalidResponse)
		}

		if *status.Message != protocol.AuthPCSWaitingCookiesValue && *status.Message != protocol.AuthPCSNoCookiesValue {
			return nativeResponseError(operation, operation.response, errNativeAuthInput, Provider)
		}

		err = sdk.nativePCSWait(ctx, operation)
		if err != nil {
			return err
		}
	}

	return nativeResponseError(operation, operation.response, errNativeAuthInput, Timeout)
}

func (sdk *SDK) nativePCSRequest(ctx context.Context, operation *nativeAuthOperation,
	service string, derived bool,
) (*auth.AuthPCSResponse, error) {
	input := auth.AuthPCSRequest{AppName: service, DerivedFromUserAction: derived}
	params := authapi.RequestAuthPCSParams(nativeAuthParams(operation.state))

	request, err := nativeEncodedRequest(input, func(body io.Reader) (*http.Request, error) {
		return nativeGeneratedRequest(authapi.NewRequestAuthPCSRequestWithBody(
			operation.state.Auth.SetupServiceURL, &params,
			nativeJSONMedia(), body))
	})
	if err != nil {
		return nil, newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, request, nativeSetupJSONHeaders(operation.state))
	if err != nil {
		return nil, err
	}

	err = nativeRequireSuccess(operation, response)
	if err != nil {
		return nil, err
	}

	var result auth.AuthPCSResponse

	err = nativeDecodeAuthObject(response.Body, &result)
	if err != nil {
		return nil, nativeResponseError(operation, response, err, InvalidResponse)
	}

	return &result, nil
}

func nativeSetupJSONHeaders(state NativeAuthState) http.Header {
	headers := requestHeaders(state.Auth.Headers)
	headers.Set(protocol.AuthHTTPContentTypeName, nativeJSONMedia())

	return headers
}
