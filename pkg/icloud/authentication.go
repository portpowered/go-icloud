package icloud

import (
	"context"
	"errors"
	"github.com/oapi-codegen/nullable"
	"maps"
	"net/url"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/internal/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

var (
	errSavedToken       = errors.New("saved session token is required")
	errUntrustedSession = errors.New("session requires interactive authentication")
	errUpdatedTerms     = errors.New("account requires updated terms acceptance")
)

type authResumeState struct {
	auth       AuthContext
	trustToken string
	responses  []ResponseMetadata
	response   *webtransport.AuthResponse
	refreshed  bool
	country    nullable.Nullable[string]
}

// ResumeSession validates cached cookies, then refreshes a rejected or untrusted session token.
// It returns copied credential updates; no password, MFA delivery, or terms acceptance is performed.
func (sdk *SDK) ResumeSession(ctx context.Context, request ResumeSessionRequest) (*ResumeSessionResult, error) {
	const operation = "ResumeSession"

	state := authResumeState{auth: savedAuthContext(request.Auth), trustToken: request.TrustToken,
		responses: nil, response: nil, refreshed: false, country: maps.Clone(request.AccountCountryCode)}
	if state.auth.SessionToken == nil || *state.auth.SessionToken == "" {
		return nil, newClientError(operation, Unauthorized, 0, nil, nil, errSavedToken)
	}

	state.auth = bindAuthCookies(state.auth)

	cookies, err := webtransport.NewCookieState(authCookies(state.auth.Cookies))
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	reused, err := sdk.reuseAuthCookie(ctx, request, &state, cookies)
	if err != nil {
		return nil, err
	}

	if !reused {
		err = sdk.refreshAuthToken(ctx, &state, cookies)
		if err != nil {
			return nil, err
		}
	}

	return state.result(request.AllowUntrusted)
}

func (state *authResumeState) result(allowUntrusted bool) (*ResumeSessionResult, error) {
	if state.refreshed && authTrue(state.response.Data.TermsUpdateNeeded) {
		return nil, state.discoveryFailure(errUpdatedTerms)
	}

	if !authTrue(state.response.Data.HsaTrustedBrowser) && !allowUntrusted {
		return nil, state.discoveryFailure(errUntrustedSession)
	}

	result := projectAuthResult(state.auth, state.trustToken, state.response, state.responses)

	result.AccountCountryCode = maps.Clone(state.country)

	if !result.AccountCountryCode.IsSpecified() {
		result.AccountCountryCode.SetNull()
	}

	return result, nil
}

func (sdk *SDK) reuseAuthCookie(ctx context.Context, request ResumeSessionRequest,
	state *authResumeState, cookies *webtransport.CookieState,
) (bool, error) {
	if request.ForceRefresh || !hasWebAuthCookie(state.auth) {
		return false, nil
	}

	response, err := sdk.web.ValidateAuthSession(ctx, state.auth.SetupServiceURL,
		requestHeaders(state.auth.Headers), cookies)
	if err != nil {
		if !authProviderFailure(err) {
			return false, adaptFailure("ResumeSession", err)
		}

		failure := adaptFailure("ResumeSession", err)
		metadata := ResponseMetadata{CookieScopeURL: failure.CookieScopeURL(), StatusCode: failure.StatusCode(),
			Headers: failure.ResponseHeaders()}
		state.recordMetadata(metadata)

		return false, nil
	}

	state.response = response
	state.recordMetadata(publicMetadata(response.Response))

	return request.AllowUntrusted || authTrue(response.Data.HsaTrustedBrowser), nil
}

func (sdk *SDK) refreshAuthToken(ctx context.Context,
	state *authResumeState, cookies *webtransport.CookieState,
) error {
	country := maps.Clone(state.country)
	if !country.IsSpecified() {
		country.SetNull()
	}

	input := auth.AuthTokenLoginRequest{AccountCountryCode: country, DsWebAuthToken: *state.auth.SessionToken,
		ExtendedLogin: true, TrustToken: state.trustToken}

	response, err := sdk.web.LoginAuthToken(ctx, state.auth.SetupServiceURL,
		requestHeaders(state.auth.Headers), cookies, input)
	if err != nil {
		failure := adaptFailure("ResumeSession", err)

		failure.prior = append([]ResponseMetadata(nil), state.responses...)

		return failure
	}

	state.response = response
	state.refreshed = true
	state.recordMetadata(publicMetadata(response.Response))

	return nil
}

func (state *authResumeState) recordMetadata(metadata ResponseMetadata) {
	state.auth = applySessionResponse(state.auth, metadata)

	headers := requestHeaders(metadata.Headers)

	if value := headers.Get(protocol.AuthHTTPXAppleSessionTokenName); value != "" {
		state.auth.SessionToken = &value
	}

	if value := headers.Get(protocol.AuthHTTPXAppleTwoSVTrustTokenName); value != "" {
		state.trustToken = value
	}

	if value := headers.Get(protocol.AuthHTTPXAppleIDAccountCountryName); value != "" {
		state.country.Set(value)
	}

	state.responses = append(state.responses, metadata)
}

func (state *authResumeState) discoveryFailure(cause error) *ClientError {
	failure := authDiscoveryFailure("ResumeSession", cause, state.response)
	failure.prior = append([]ResponseMetadata(nil), state.responses[:len(state.responses)-1]...)

	return failure
}

func hasWebAuthCookie(state AuthContext) bool {
	for _, cookie := range state.Cookies {
		if cookie.Name == protocol.AuthWebAuthCookieNameValue && cookie.Value != "" {
			return true
		}
	}

	return false
}

func authProviderFailure(err error) bool {
	var failure *webtransport.ResponseError

	return errors.As(err, &failure) && failure.Stage == webtransport.Provider
}

func authTrue(value *bool) bool { return value != nil && *value }

func authDiscoveryFailure(operation string, cause error, response *webtransport.AuthResponse) *ClientError {
	kind := AuthenticationRequired
	if errors.Is(cause, errUpdatedTerms) {
		kind = TermsRequired
	}

	result := newClientError(operation, kind, response.Response.Status, response.Response.Body,
		responseHeaders(response.Response.Headers), cause)
	result.cookieScopeURL = response.Response.CookieScopeURL

	return result
}

func projectAuthResult(state AuthContext, trustToken string, response *webtransport.AuthResponse,
	responses []ResponseMetadata,
) *ResumeSessionResult {
	data := response.Data
	if data.DsInfo != nil && data.DsInfo.Dsid != nil {
		state.AccountID = *data.DsInfo.Dsid
	}

	state.AccountServiceURL = ""

	state.LegacyRemindersServiceURL = ""
	state.RemindersServiceURL = ""
	state.PhotosServiceURL = ""

	if data.Webservices != nil {
		state.AccountServiceURL = authServiceURL(data.Webservices.Account)
		state.LegacyRemindersServiceURL = authServiceURL(data.Webservices.Reminders)
		state.RemindersServiceURL = authServiceURL(data.Webservices.Ckdatabasews)
		state.PhotosServiceURL = authServiceURL(data.Webservices.Ckdatabasews)
		state.DriveServiceURL = authServiceURL(data.Webservices.Drivews)
		state.DriveDocumentServiceURL = authServiceURL(data.Webservices.Docws)
		state.FindMyServiceURL = authServiceURL(data.Webservices.Findme)
	}

	trusted := authTrue(data.HsaTrustedBrowser)
	required := authTrue(data.HsaChallengeRequired) || !trusted

	version := 0
	if data.DsInfo != nil && data.DsInfo.HsaVersion != nil {
		version = *data.DsInfo.HsaVersion
	}

	return &ResumeSessionResult{Auth: state, TrustToken: trustToken, TrustedSession: trusted,
		RequiresTwoFactor: required && version == 2, RequiresTwoStep: required && version >= 1,
		AccountData: append([]byte(nil), response.Response.Body...), Responses: responses, AccountCountryCode: nil}
}

func authServiceURL(service *auth.AuthService) string {
	if service == nil || service.Url == nil {
		return ""
	}

	return *service.Url
}

func savedAuthContext(credentials SavedSessionCredentials) AuthContext {
	var state AuthContext

	state.ClientID = credentials.ClientID
	state.SetupServiceURL = credentials.SetupServiceURL
	state.SessionToken = &credentials.SessionToken
	state.Headers = credentials.Headers
	state.Cookies = credentials.Cookies
	state.ChinaMainland = credentials.ChinaMainland
	build, mastering := protocol.AuthClientBuildNumberValue, protocol.AuthClientMasteringNumberValue
	state.ClientBuildNumber = &build
	state.ClientMasteringNumber = &mastering

	return cloneDriveAuth(state)
}

func bindAuthCookies(state AuthContext) AuthContext {
	target, err := url.Parse(state.SetupServiceURL)
	if err != nil {
		return state
	}

	for index := range state.Cookies {
		cookie := &state.Cookies[index]
		if cookie.Domain == "" {
			cookie.Domain = target.Hostname()
			cookie.HostOnly = true
		}

		if cookie.Path == "" {
			cookie.Path = driveDefaultCookiePath(protocol.AuthValidateAuthSessionPath)
		}
	}

	return bindSessionCookies(state, state.SetupServiceURL)
}
