package icloud

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
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

		return sdk.nativeTokenTerms(ctx, operation, input, response.Data, requireTrust)
	}

	return nativeTokenAccepted(operation, response.Data, requireTrust), nil
}

func (sdk *SDK) nativeTokenTerms(ctx context.Context, operation *nativeAuthOperation,
	input auth.AuthTokenLoginRequest, account auth.AuthAccountResponse, requireTrust bool,
) (bool, error) {
	accepted, err := sdk.nativeAcceptTerms(ctx, operation, input, account)
	if err != nil || !accepted {
		return accepted, err
	}

	var repaired auth.AuthAccountResponse

	err = nativeDecodeAuthObject(operation.state.AccountData, &repaired)
	if err != nil {
		return false, nativeResponseError(operation, operation.response, err, InvalidResponse)
	}

	return nativeTokenAccepted(operation, repaired, requireTrust), nil
}

func nativeTokenAccepted(operation *nativeAuthOperation, account auth.AuthAccountResponse,
	requireTrust bool,
) bool {
	if requireTrust && !authTrue(account.HsaTrustedBrowser) {
		return false
	}

	operation.state.RequiresMFA = false
	operation.state.CodeRequested = false
	operation.state.DeliveryMethod = TwoFactorDeliveryUnknown
	operation.state.DeliveryNotice = nil
	operation.state.Challenge = emptyNativeAuthChallenge()

	return true
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
	case AccountLocked, TermsRequired, InvalidResponse, Canceled, Timeout, Transport, Configuration, Busy:
		return false
	case AuthenticationRequired, Closed, Forbidden, NoDevices, NotDirectory, NotFound, NotInTrash,
		Provider, RateLimited, Unauthorized, Unavailable:
		return true
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
