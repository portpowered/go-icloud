package webtransport

import (
	"errors"
	"net/http"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/httpboundary"
)

var errAuthenticationMedia = errors.New("authentication media is incompatible with the typed operation")

func authenticationCallMedia(call AuthenticationCall) string {
	switch call.(type) {
	case InitAuthSRPCall, CompleteAuthSRPCall, LoginAuthTokenCall, LoginAuthCredentialsCall,
		RequestAuthSMSCall, VerifyAuthSMSCall, VerifyAuthTrustedCodeCall, VerifyAuthSecurityKeyCall,
		SendAuthVerificationCodeCall, ValidateAuthVerificationCodeCall,
		GetAuthTermsCall, AcceptAuthTermsCall, RequestAuthPCSCall,
		AuthBridgeStep0Call, AuthBridgeStep2Call, AuthBridgeStep4Call, AuthBridgeStep6Call,
		ValidateAuthBridgeCodeCall:
		return string(httpboundary.HTTPJSONMediaApplicationJSON)
	case LogoutAuthSessionCall:
		return protocol.AuthLogoutContentTypeValue
	case AuthorizeAuthSignInCall, GetAuthChallengeCall, TrustAuthSessionCall,
		ListAuthTrustedDevicesCall, GetAuthWebAccessStateCall, EnableAuthPCSConsentCall:
		return ""
	default:
		return ""
	}
}

func validateAuthenticationMedia(initial, caller http.Header, expected string) error {
	for _, headers := range []http.Header{initial, caller} {
		for _, value := range headers.Values(protocol.AuthHTTPContentTypeName) {
			if value == "" {
				continue
			}
			if expected == "" {
				if value != string(httpboundary.HTTPJSONMediaApplicationJSON) {
					return errAuthenticationMedia
				}
			} else if value != expected {
				return errAuthenticationMedia
			}
		}
	}
	return nil
}

func authenticationHeaders(request *http.Request, caller http.Header, media string) {
	initialMedia := request.Header.Get(protocol.AuthHTTPContentTypeName)
	request.Header = callerHeaders(caller)
	if media != "" {
		request.Header.Set(protocol.AuthHTTPContentTypeName, media)
	} else if initialMedia != "" || request.Header.Get(protocol.AuthHTTPContentTypeName) != "" {
		request.Header.Set(protocol.AuthHTTPContentTypeName, string(httpboundary.HTTPJSONMediaApplicationJSON))
	} else {
		request.Header.Del(protocol.AuthHTTPContentTypeName)
	}
}
