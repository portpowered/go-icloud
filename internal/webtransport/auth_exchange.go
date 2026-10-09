package webtransport

import (
	"context"
	"errors"
	"net/http"

	"github.com/portpowered/go-icloud/internal/protocol"
)

var errUnknownAuthRoute = errors.New("authentication route is absent from the protocol schema")

// ExchangeAuthentication sends a generated authentication request and retains exact response evidence.
// Conflict and precondition responses are returned so the specific verifier can interpret their verdict.
func (client *Client) ExchangeAuthentication(ctx context.Context, request *http.Request,
	headers http.Header, cookies *CookieState,
) (*BytesResponse, error) {
	if request == nil || request.URL == nil || request.URL.User != nil || request.URL.Fragment != "" ||
		!knownAuthenticationRoute(request) {
		return nil, failure(Configuration, errUnknownAuthRoute, nil, nil)
	}

	originErr := validateOrigin(request.URL.Scheme + "://" + request.URL.Host)
	if originErr != nil {
		return nil, failure(Configuration, originErr, nil, nil)
	}

	request = request.WithContext(ctx)
	contentType := request.Header.Get(protocol.AuthHTTPContentTypeName)

	request.Header = headers.Clone()

	if request.Header.Get(protocol.AuthHTTPContentTypeName) == "" && contentType != "" {
		request.Header.Set(protocol.AuthHTTPContentTypeName, contentType)
	}

	policy := authenticationContent
	if authenticationBridgeRoute(request.URL.Path) {
		policy = authenticationRawContent
	}

	return client.readPrepared(request, policy, cookies)
}

func authenticationBridgeRoute(path string) bool {
	switch path {
	case protocol.AuthBridgeStep0Path, protocol.AuthBridgeStep2Path, protocol.AuthBridgeStep4Path,
		protocol.AuthBridgeStep6Path, protocol.AuthValidateAuthBridgeCodePath:
		return true
	default:
		return false
	}
}

func knownAuthenticationRoute(request *http.Request) bool {
	method := request.Method

	switch request.URL.Path {
	case protocol.AuthorizeAuthSignInPath, protocol.AuthGetAuthChallengePath,
		protocol.AuthTrustAuthSessionPath, protocol.AuthListAuthTrustedDevicesPath,
		protocol.AuthGetAuthTermsPath, protocol.AuthAcceptAuthTermsPath:
		return method == http.MethodGet
	case protocol.AuthRequestAuthSMSPath:
		return method == protocol.AuthRequestAuthSMSMethod
	case protocol.AuthInitAuthSRPPath, protocol.AuthCompleteAuthSRPPath,
		protocol.AuthVerifyAuthSMSPath, protocol.AuthVerifyAuthTrustedCodePath,
		protocol.AuthVerifyAuthSecurityKeyPath, protocol.AuthSendAuthVerificationCodePath,
		protocol.AuthValidateAuthVerificationCodePath, protocol.AuthLogoutAuthSessionPath,
		protocol.AuthGetAuthWebAccessStatePath, protocol.AuthEnableAuthPCSConsentPath,
		protocol.AuthRequestAuthPCSPath, protocol.AuthBridgeStep0Path,
		protocol.AuthBridgeStep2Path, protocol.AuthBridgeStep4Path,
		protocol.AuthBridgeStep6Path, protocol.AuthValidateAuthBridgeCodePath, protocol.AuthLoginAuthTokenPath:
		return method == http.MethodPost
	default:
		return false
	}
}
