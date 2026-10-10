package webtransport

import (
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/authapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

// AuthenticationCall selects a schema-owned authentication operation.
// Implementations in this package bind a generated parameter and body type to its route.
// Callers supply account origins and credentials through the concrete call fields.
type AuthenticationCall interface{ authenticationCall() }

// AuthorizeAuthSignInCall carries the generated parameters for AuthorizeAuthSignIn.
type AuthorizeAuthSignInCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.AuthorizeAuthSignInParams
}

func (AuthorizeAuthSignInCall) authenticationCall() {}

// GetAuthChallengeCall carries the generated parameters for GetAuthChallenge.
type GetAuthChallengeCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.GetAuthChallengeParams
}

func (GetAuthChallengeCall) authenticationCall() {}

// TrustAuthSessionCall carries the generated parameters for TrustAuthSession.
type TrustAuthSessionCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.TrustAuthSessionParams
}

func (TrustAuthSessionCall) authenticationCall() {}

// ListAuthTrustedDevicesCall carries the generated parameters for ListAuthTrustedDevices.
type ListAuthTrustedDevicesCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.ListAuthTrustedDevicesParams
}

func (ListAuthTrustedDevicesCall) authenticationCall() {}

// GetAuthWebAccessStateCall carries the generated parameters for GetAuthWebAccessState.
type GetAuthWebAccessStateCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.GetAuthWebAccessStateParams
}

func (GetAuthWebAccessStateCall) authenticationCall() {}

// EnableAuthPCSConsentCall carries the generated parameters for EnableAuthPCSConsent.
type EnableAuthPCSConsentCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.EnableAuthPCSConsentParams
}

func (EnableAuthPCSConsentCall) authenticationCall() {}

// InitAuthSRPCall carries the generated parameters and authentication body for InitAuthSRP.
type InitAuthSRPCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.InitAuthSRPParams
	Body   auth.AuthSRPInitRequest
}

func (InitAuthSRPCall) authenticationCall() {}

// CompleteAuthSRPCall carries the generated parameters and authentication body for CompleteAuthSRP.
type CompleteAuthSRPCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.CompleteAuthSRPParams
	Body   auth.AuthSRPCompleteRequest
}

func (CompleteAuthSRPCall) authenticationCall() {}

// LoginAuthTokenCall carries the generated parameters and authentication body for LoginAuthToken.
type LoginAuthTokenCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.LoginAuthTokenParams
	Body   auth.AuthTokenLoginRequest
}

func (LoginAuthTokenCall) authenticationCall() {}

// LoginAuthCredentialsCall carries the generated parameters and authentication body for LoginAuthToken.
type LoginAuthCredentialsCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.LoginAuthTokenParams
	Body   auth.AuthCredentialsLoginRequest
}

func (LoginAuthCredentialsCall) authenticationCall() {}

// RequestAuthSMSCall carries the generated parameters and authentication body for RequestAuthSMS.
type RequestAuthSMSCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.RequestAuthSMSParams
	Body   auth.AuthSMSRequest
}

func (RequestAuthSMSCall) authenticationCall() {}

// VerifyAuthSMSCall carries the generated parameters and authentication body for VerifyAuthSMS.
type VerifyAuthSMSCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.VerifyAuthSMSParams
	Body   auth.AuthSMSVerificationRequest
}

func (VerifyAuthSMSCall) authenticationCall() {}

// VerifyAuthTrustedCodeCall carries the generated parameters and authentication body for VerifyAuthTrustedCode.
type VerifyAuthTrustedCodeCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.VerifyAuthTrustedCodeParams
	Body   auth.AuthTrustedCodeRequest
}

func (VerifyAuthTrustedCodeCall) authenticationCall() {}

// VerifyAuthSecurityKeyCall carries the generated parameters and authentication body for VerifyAuthSecurityKey.
type VerifyAuthSecurityKeyCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.VerifyAuthSecurityKeyParams
	Body   auth.AuthWebAuthnAssertion
}

func (VerifyAuthSecurityKeyCall) authenticationCall() {}

// SendAuthVerificationCodeCall carries the generated parameters and authentication body for SendAuthVerificationCode.
type SendAuthVerificationCodeCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.SendAuthVerificationCodeParams
	Body   auth.AuthTrustedDevice
}

func (SendAuthVerificationCodeCall) authenticationCall() {}

// ValidateAuthVerificationCodeCall carries the generated parameters and authentication body for ValidateAuthVerificationCode.
type ValidateAuthVerificationCodeCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.ValidateAuthVerificationCodeParams
	Body   auth.AuthTrustedDevice
}

func (ValidateAuthVerificationCodeCall) authenticationCall() {}

// GetAuthTermsCall carries the generated parameters and authentication body for GetAuthTerms.
type GetAuthTermsCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.GetAuthTermsParams
	Body   auth.AuthGetTermsRequest
}

func (GetAuthTermsCall) authenticationCall() {}

// AcceptAuthTermsCall carries the generated parameters and authentication body for AcceptAuthTerms.
type AcceptAuthTermsCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.AcceptAuthTermsParams
	Body   auth.AuthAcceptTermsRequest
}

func (AcceptAuthTermsCall) authenticationCall() {}

// LogoutAuthSessionCall carries the generated parameters and authentication body for LogoutAuthSession.
type LogoutAuthSessionCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.LogoutAuthSessionParams
	Body   auth.AuthLogoutRequest
}

func (LogoutAuthSessionCall) authenticationCall() {}

// RequestAuthPCSCall carries the generated parameters and authentication body for RequestAuthPCS.
type RequestAuthPCSCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.RequestAuthPCSParams
	Body   auth.AuthPCSRequest
}

func (RequestAuthPCSCall) authenticationCall() {}

// AuthBridgeStep0Call carries the generated parameters and authentication body for AuthBridgeStep0.
type AuthBridgeStep0Call struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.AuthBridgeStep0Params
	Body   auth.AuthBridgeStartRequest
}

func (AuthBridgeStep0Call) authenticationCall() {}

// AuthBridgeStep2Call carries the generated parameters and authentication body for AuthBridgeStep2.
type AuthBridgeStep2Call struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.AuthBridgeStep2Params
	Body   auth.AuthBridgeStepRequest
}

func (AuthBridgeStep2Call) authenticationCall() {}

// AuthBridgeStep4Call carries the generated parameters and authentication body for AuthBridgeStep4.
type AuthBridgeStep4Call struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.AuthBridgeStep4Params
	Body   auth.AuthBridgeStepRequest
}

func (AuthBridgeStep4Call) authenticationCall() {}

// AuthBridgeStep6Call carries the generated parameters and authentication body for AuthBridgeStep6.
type AuthBridgeStep6Call struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.AuthBridgeStep6Params
	Body   auth.AuthBridgeStepRequest
}

func (AuthBridgeStep6Call) authenticationCall() {}

// ValidateAuthBridgeCodeCall carries the generated parameters and authentication body for ValidateAuthBridgeCode.
type ValidateAuthBridgeCodeCall struct {
	// Origin is the caller-owned HTTPS account origin.
	Origin string
	Params *authapi.ValidateAuthBridgeCodeParams
	Body   auth.AuthBridgeCodeRequest
}

func (ValidateAuthBridgeCodeCall) authenticationCall() {}
