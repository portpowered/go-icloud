package icloud

import "context"

// GetAuthenticationStatus checks saved authentication without starting a password or MFA flow.
func (sdk *SDK) GetAuthenticationStatus(ctx context.Context,
	request NativeAuthRequest,
) (*AuthenticationStatusResult, error) {
	operation, err := newNativeAuthOperation(ctx, "GetAuthenticationStatus", request.Auth, request.State)
	if err != nil {
		return nil, err
	}

	result := AuthenticationStatusResult{State: operation.state, Authenticated: false, TrustedSession: false,
		RequiresTwoFactor: false, RequiresTwoStep: false, Responses: nil}
	if !nativeHasToken(operation.state) || !hasWebAuthCookie(operation.state.Auth) {
		result.State = clearNativeDerivedState(operation.state)
		result.Responses = []ResponseMetadata{}

		return &result, nil
	}

	accepted, err := sdk.nativeReuseCookie(ctx, operation, true)
	if err != nil {
		return nil, err
	}

	if !accepted {
		result.State = clearNativeDerivedState(operation.state)
		result.Responses = cloneDriveResponses(operation.responses)

		return &result, nil
	}

	projected, err := nativeAuthResult(operation)
	if err != nil {
		return nil, err
	}

	result.State = projected.State
	result.Authenticated = true
	result.TrustedSession = projected.TrustedSession
	result.RequiresTwoFactor = projected.RequiresTwoFactor
	result.RequiresTwoStep = projected.RequiresTwoStep
	result.Responses = projected.Responses

	return &result, nil
}

func clearNativeDerivedState(state NativeAuthState) NativeAuthState {
	state = cloneNativeAuthState(state)
	state.AccountData = []byte("{}")
	state.Challenge = emptyNativeAuthChallenge()
	state.RequiresMFA = false
	state.CodeRequested = false
	state.DeliveryMethod = TwoFactorDeliveryUnknown
	state.DeliveryNotice = nil
	state.Auth.AccountID = ""
	state.Auth.AccountServiceURL = ""
	state.Auth.DriveServiceURL = ""
	state.Auth.DriveDocumentServiceURL = ""
	state.Auth.FindMyServiceURL = ""
	state.Auth.PhotosServiceURL = ""
	state.Auth.PhotosUploadServiceURL = ""
	state.Auth.SharedPhotosServiceURL = ""
	state.Auth.RemindersServiceURL = ""
	state.Auth.LegacyRemindersServiceURL = ""

	return state
}
