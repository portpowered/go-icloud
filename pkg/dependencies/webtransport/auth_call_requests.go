package webtransport

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/authapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/httpboundary"
)

// ValidateAuthenticationCall checks that the typed operation can prepare its generated request.
// ExchangeAuthentication independently constructs the request it sends from the same typed call.
func ValidateAuthenticationCall(call AuthenticationCall) error {
	request, err := buildAuthenticationRequest(call)
	if err != nil {
		return err
	}
	return validateAuthenticationMedia(request.Header, nil, authenticationCallMedia(call))
}

func buildAuthenticationRequest(call AuthenticationCall) (*http.Request, error) {
	switch call.(type) {
	case AuthorizeAuthSignInCall, GetAuthChallengeCall, TrustAuthSessionCall, ListAuthTrustedDevicesCall, GetAuthWebAccessStateCall, EnableAuthPCSConsentCall:
		return buildRetrievalAuthentication(call)
	case InitAuthSRPCall, CompleteAuthSRPCall, LoginAuthTokenCall, LoginAuthCredentialsCall:
		return buildPasswordAuthentication(call)
	case RequestAuthSMSCall, VerifyAuthSMSCall, VerifyAuthTrustedCodeCall, VerifyAuthSecurityKeyCall, SendAuthVerificationCodeCall, ValidateAuthVerificationCodeCall:
		return buildVerificationAuthentication(call)
	case GetAuthTermsCall, AcceptAuthTermsCall, LogoutAuthSessionCall, RequestAuthPCSCall:
		return buildSetupAuthentication(call)
	case AuthBridgeStep0Call, AuthBridgeStep2Call, AuthBridgeStep4Call, AuthBridgeStep6Call, ValidateAuthBridgeCodeCall:
		return buildBridgeAuthentication(call)
	default:
		return nil, errUnknownAuthRoute
	}
}

type authenticationBodyBuilder func(io.Reader) (*http.Request, error)

func buildAuthenticationBody(input any, builder authenticationBodyBuilder) (*http.Request, error) {
	body, err := EncodeAuthenticationRequest(input)
	if err != nil {
		return nil, fmt.Errorf("prepare authentication body: %w", err)
	}
	request, err := builder(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("prepare authentication request: %w", err)
	}
	return request, nil
}

func buildRetrievalAuthentication(call AuthenticationCall) (*http.Request, error) {
	switch input := call.(type) {
	case AuthorizeAuthSignInCall:
		return authapi.NewAuthorizeAuthSignInRequest(input.Origin, input.Params)
	case GetAuthChallengeCall:
		return authapi.NewGetAuthChallengeRequest(input.Origin, input.Params)
	case TrustAuthSessionCall:
		return authapi.NewTrustAuthSessionRequest(input.Origin, input.Params)
	case ListAuthTrustedDevicesCall:
		return authapi.NewListAuthTrustedDevicesRequest(input.Origin, input.Params)
	case GetAuthWebAccessStateCall:
		return authapi.NewGetAuthWebAccessStateRequest(input.Origin, input.Params)
	case EnableAuthPCSConsentCall:
		return authapi.NewEnableAuthPCSConsentRequest(input.Origin, input.Params)
	default:
		return nil, errUnknownAuthRoute
	}
}

func buildPasswordAuthentication(call AuthenticationCall) (*http.Request, error) {
	switch input := call.(type) {
	case InitAuthSRPCall:
		return buildAuthenticationBody(input.Body, func(body io.Reader) (*http.Request, error) {
			return authapi.NewInitAuthSRPRequestWithBody(input.Origin, input.Params, string(httpboundary.HTTPJSONMediaApplicationJSON), body)
		})
	case CompleteAuthSRPCall:
		return buildAuthenticationBody(input.Body, func(body io.Reader) (*http.Request, error) {
			return authapi.NewCompleteAuthSRPRequestWithBody(input.Origin, input.Params, string(httpboundary.HTTPJSONMediaApplicationJSON), body)
		})
	case LoginAuthTokenCall:
		return buildAuthenticationBody(input.Body, func(body io.Reader) (*http.Request, error) {
			return authapi.NewLoginAuthTokenRequestWithBody(input.Origin, input.Params, string(httpboundary.HTTPJSONMediaApplicationJSON), body)
		})
	case LoginAuthCredentialsCall:
		return buildAuthenticationBody(input.Body, func(body io.Reader) (*http.Request, error) {
			return authapi.NewLoginAuthTokenRequestWithBody(input.Origin, input.Params, string(httpboundary.HTTPJSONMediaApplicationJSON), body)
		})
	default:
		return nil, errUnknownAuthRoute
	}
}

func buildVerificationAuthentication(call AuthenticationCall) (*http.Request, error) {
	switch input := call.(type) {
	case RequestAuthSMSCall:
		return buildAuthenticationBody(input.Body, func(body io.Reader) (*http.Request, error) {
			return authapi.NewRequestAuthSMSRequestWithBody(input.Origin, input.Params, string(httpboundary.HTTPJSONMediaApplicationJSON), body)
		})
	case VerifyAuthSMSCall:
		return buildAuthenticationBody(input.Body, func(body io.Reader) (*http.Request, error) {
			return authapi.NewVerifyAuthSMSRequestWithBody(input.Origin, input.Params, string(httpboundary.HTTPJSONMediaApplicationJSON), body)
		})
	case VerifyAuthTrustedCodeCall:
		return buildAuthenticationBody(input.Body, func(body io.Reader) (*http.Request, error) {
			return authapi.NewVerifyAuthTrustedCodeRequestWithBody(input.Origin, input.Params, string(httpboundary.HTTPJSONMediaApplicationJSON), body)
		})
	case VerifyAuthSecurityKeyCall:
		return buildAuthenticationBody(input.Body, func(body io.Reader) (*http.Request, error) {
			return authapi.NewVerifyAuthSecurityKeyRequestWithBody(input.Origin, input.Params, string(httpboundary.HTTPJSONMediaApplicationJSON), body)
		})
	case SendAuthVerificationCodeCall:
		return buildAuthenticationBody(input.Body, func(body io.Reader) (*http.Request, error) {
			return authapi.NewSendAuthVerificationCodeRequestWithBody(input.Origin, input.Params, string(httpboundary.HTTPJSONMediaApplicationJSON), body)
		})
	case ValidateAuthVerificationCodeCall:
		return buildAuthenticationBody(input.Body, func(body io.Reader) (*http.Request, error) {
			return authapi.NewValidateAuthVerificationCodeRequestWithBody(input.Origin, input.Params, string(httpboundary.HTTPJSONMediaApplicationJSON), body)
		})
	default:
		return nil, errUnknownAuthRoute
	}
}

func buildSetupAuthentication(call AuthenticationCall) (*http.Request, error) {
	switch input := call.(type) {
	case GetAuthTermsCall:
		return buildAuthenticationBody(input.Body, func(body io.Reader) (*http.Request, error) {
			return authapi.NewGetAuthTermsRequestWithBody(input.Origin, input.Params, string(httpboundary.HTTPJSONMediaApplicationJSON), body)
		})
	case AcceptAuthTermsCall:
		return buildAuthenticationBody(input.Body, func(body io.Reader) (*http.Request, error) {
			return authapi.NewAcceptAuthTermsRequestWithBody(input.Origin, input.Params, string(httpboundary.HTTPJSONMediaApplicationJSON), body)
		})
	case LogoutAuthSessionCall:
		return buildAuthenticationBody(input.Body, func(body io.Reader) (*http.Request, error) {
			return authapi.NewLogoutAuthSessionRequestWithBody(input.Origin, input.Params, protocol.AuthLogoutContentTypeValue, body)
		})
	case RequestAuthPCSCall:
		return buildAuthenticationBody(input.Body, func(body io.Reader) (*http.Request, error) {
			return authapi.NewRequestAuthPCSRequestWithBody(input.Origin, input.Params, string(httpboundary.HTTPJSONMediaApplicationJSON), body)
		})
	default:
		return nil, errUnknownAuthRoute
	}
}

func buildBridgeAuthentication(call AuthenticationCall) (*http.Request, error) {
	switch input := call.(type) {
	case AuthBridgeStep0Call:
		return buildAuthenticationBody(input.Body, func(body io.Reader) (*http.Request, error) {
			return authapi.NewAuthBridgeStep0RequestWithBody(input.Origin, input.Params, string(httpboundary.HTTPJSONMediaApplicationJSON), body)
		})
	case AuthBridgeStep2Call:
		return buildAuthenticationBody(input.Body, func(body io.Reader) (*http.Request, error) {
			return authapi.NewAuthBridgeStep2RequestWithBody(input.Origin, input.Params, string(httpboundary.HTTPJSONMediaApplicationJSON), body)
		})
	case AuthBridgeStep4Call:
		return buildAuthenticationBody(input.Body, func(body io.Reader) (*http.Request, error) {
			return authapi.NewAuthBridgeStep4RequestWithBody(input.Origin, input.Params, string(httpboundary.HTTPJSONMediaApplicationJSON), body)
		})
	case AuthBridgeStep6Call:
		return buildAuthenticationBody(input.Body, func(body io.Reader) (*http.Request, error) {
			return authapi.NewAuthBridgeStep6RequestWithBody(input.Origin, input.Params, string(httpboundary.HTTPJSONMediaApplicationJSON), body)
		})
	case ValidateAuthBridgeCodeCall:
		return buildAuthenticationBody(input.Body, func(body io.Reader) (*http.Request, error) {
			return authapi.NewValidateAuthBridgeCodeRequestWithBody(input.Origin, input.Params, string(httpboundary.HTTPJSONMediaApplicationJSON), body)
		})
	default:
		return nil, errUnknownAuthRoute
	}
}
