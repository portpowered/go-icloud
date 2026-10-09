package icloud

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/portpowered/go-icloud/internal/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

func (sdk *SDK) nativeReuseCookie(ctx context.Context, operation *nativeAuthOperation,
	allowUntrusted bool,
) (bool, error) {
	if !hasWebAuthCookie(operation.state.Auth) {
		return false, nil
	}

	response, err := sdk.web.ValidateAuthSession(ctx, operation.state.Auth.SetupServiceURL,
		requestHeaders(operation.state.Auth.Headers), operation.cookies)
	if err != nil {
		return false, nativeRecordFailure(operation, err, true)
	}

	operation.record(response.Response)
	sdk.nativeAccountDiscovery(operation, response)

	return allowUntrusted || authTrue(response.Data.HsaTrustedBrowser), nil
}

func (sdk *SDK) nativeLoginToken(ctx context.Context, operation *nativeAuthOperation,
	requireTrust, acceptTerms bool,
) (bool, error) {
	if !nativeHasToken(operation.state) {
		return false, nil
	}

	input := auth.AuthTokenLoginRequest{AccountCountryCode: operation.state.AccountCountryCode,
		DsWebAuthToken: *operation.state.Auth.SessionToken, ExtendedLogin: true, TrustToken: operation.state.TrustToken}

	response, err := sdk.web.LoginAuthToken(ctx, operation.state.Auth.SetupServiceURL,
		requestHeaders(operation.state.Auth.Headers), operation.cookies, input)
	if err != nil {
		return false, nativeRecordFailure(operation, err, false)
	}

	operation.record(response.Response)
	sdk.nativeAccountDiscovery(operation, response)

	if authTrue(response.Data.TermsUpdateNeeded) {
		if !acceptTerms {
			return false, nativeResponseError(operation, response.Response, errUpdatedTerms, TermsRequired)
		}

		return sdk.nativeAcceptTerms(ctx, operation, input, response.Data)
	}

	if requireTrust && !authTrue(response.Data.HsaTrustedBrowser) {
		return false, nil
	}

	operation.state.RequiresMFA = false
	operation.state.CodeRequested = false
	operation.state.DeliveryMethod = TwoFactorDeliveryUnknown
	operation.state.Challenge = initialNativeAuthState(AuthenticateRequest{}).Challenge

	return true, nil
}

func nativeRecordFailure(operation *nativeAuthOperation, err error, ignoreProvider bool) error {
	failure := operation.failure(err)
	if nativeLockedBody(failure.ResponseBody()) {
		failure.kind = AccountLocked
	}
	if failure.StatusCode() != 0 {
		operation.record(&webtransport.BytesResponse{CookieScopeURL: failure.CookieScopeURL(),
			Status: failure.StatusCode(), Headers: requestHeaders(failure.ResponseHeaders()), Body: failure.ResponseBody()})
	}

	if ignoreProvider && authProviderFailure(err) && failure.Kind() != AccountLocked {
		return nil
	}

	return failure
}

func nativeCanRetry(err error) bool {
	var failure *ClientError

	if !errors.As(err, &failure) || failure.StatusCode() == 0 {
		return false
	}
	switch failure.Kind() {
	case AccountLocked, TermsRequired, InvalidResponse, Canceled, Timeout, Transport, Configuration:
		return false
	default:
		return true
	}
}

func (sdk *SDK) nativeAccountDiscovery(operation *nativeAuthOperation, response *webtransport.AuthResponse) {
	projected := projectAuthResult(operation.state.Auth, operation.state.TrustToken, response, nil)
	operation.state.Auth = projected.Auth
	operation.state.AccountData = projected.AccountData
}

func (sdk *SDK) nativeOneFactorEligible(state NativeAuthState, service string) bool {
	var data auth.AuthAccountResponse

	if json.Unmarshal(state.AccountData, &data) != nil {
		return false
	}

	if data.Apps == nil {
		return false
	}

	app, exists := (*data.Apps)[service]

	return exists && authTrue(app.CanLaunchWithOneFactor)
}
