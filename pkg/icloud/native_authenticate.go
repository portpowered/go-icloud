package icloud

import (
	"context"
	"maps"
	"strings"

	"github.com/portpowered/go-icloud/internal/protocol"
)

// Authenticate validates saved credentials or performs native password SRP authentication.
// A pending MFA challenge is returned as caller-owned progress, without retaining the password.
func (sdk *SDK) Authenticate(ctx context.Context, request AuthenticateRequest) (*NativeAuthResult, error) {
	const name = "Authenticate"

	contextErr := ctx.Err()
	if contextErr != nil {
		return nil, driveContextFailure(name, contextErr)
	}

	state := initialNativeAuthState(request)
	if state.Auth.ClientID == "" {
		identifier, err := sdk.randomUUID()
		if err != nil {
			return nil, newClientError(name, Configuration, 0, nil, nil, err)
		}

		state.Auth.ClientID = strings.ToLower(identifier)
	}

	operation, err := newNativeAuthOperation(ctx, name, state.Auth, state)
	if err != nil {
		return nil, err
	}

	if request.AccountName == "" {
		return nil, newClientError(name, Configuration, 0, nil, nil, errNativeAuthInput)
	}

	completed, err := sdk.nativeTrySavedAuthentication(ctx, operation, request)
	if err != nil {
		return nil, err
	}

	if completed {
		return nativeAuthResult(operation)
	}

	if request.Password == "" {
		failure := newClientError(name, Unauthorized, 0, nil, nil, errNativeAuthInput)
		failure.prior = cloneDriveResponses(operation.responses)

		return nil, failure
	}

	err = sdk.nativePassword(ctx, operation, request)
	if err != nil {
		return nil, err
	}

	return nativeAuthResult(operation)
}

func initialNativeAuthState(request AuthenticateRequest) NativeAuthState {
	state := NativeAuthState{Auth: cloneDriveAuth(request.Auth), AccountName: request.AccountName,
		TrustToken: request.TrustToken, AccountCountryCode: maps.Clone(request.AccountCountryCode),
		AccountData: []byte("{}"), Challenge: emptyNativeAuthChallenge(), DeliveryMethod: TwoFactorDeliveryUnknown,
		CodeRequested: false, RequiresMFA: false, DeliveryNotice: nil, AcceptTerms: request.AcceptTerms}
	if request.SavedState != nil {
		state = cloneNativeAuthState(*request.SavedState)
		state.Auth = cloneDriveAuth(request.Auth)
	}

	state.AccountName = request.AccountName
	state.AcceptTerms = request.AcceptTerms
	if state.Auth.SetupServiceURL == "" {
		state.Auth.SetupServiceURL = protocol.AuthAccountServer0
		if state.Auth.ChinaMainland != nil && *state.Auth.ChinaMainland {
			state.Auth.SetupServiceURL = protocol.AuthAccountServer1
		}
	}

	if !state.AccountCountryCode.IsSpecified() {
		state.AccountCountryCode.SetNull()
	}

	build, mastering := protocol.AuthClientBuildNumberValue, protocol.AuthClientMasteringNumberValue
	if state.Auth.ClientBuildNumber == nil {
		state.Auth.ClientBuildNumber = &build
	}

	if state.Auth.ClientMasteringNumber == nil {
		state.Auth.ClientMasteringNumber = &mastering
	}

	return state
}

func nativeHasToken(state NativeAuthState) bool {
	return state.Auth.SessionToken != nil && *state.Auth.SessionToken != ""
}

func emptyNativeAuthChallenge() NativeAuthChallenge {
	return NativeAuthChallenge{Mode: "", PhoneNumbers: []TrustedPhoneNumber{}, SecurityKeyNames: []string{},
		AuthInitialRoute: "", HasTrustedDevices: false, AuthFactors: []string{}, BridgeBootstrap: nil,
		SecurityKeyChallenge: nil, ProviderData: nil}
}

func (sdk *SDK) nativeTrySavedAuthentication(ctx context.Context, operation *nativeAuthOperation,
	request AuthenticateRequest,
) (bool, error) {
	reused, err := sdk.nativeTryCookieAuthentication(ctx, operation, request)
	if err != nil || reused {
		return reused, err
	}

	accepted, err := sdk.nativeTryServiceAuthentication(ctx, operation, request)
	if err != nil || accepted {
		return accepted, err
	}

	return sdk.nativeTryTokenAuthentication(ctx, operation, request)
}

func (sdk *SDK) nativeTryCookieAuthentication(ctx context.Context, operation *nativeAuthOperation,
	request AuthenticateRequest,
) (bool, error) {
	if request.ForceRefresh || !nativeHasToken(operation.state) {
		return false, nil
	}

	return sdk.nativeReuseCookie(ctx, operation, request.PauseTwoFactor)
}

func (sdk *SDK) nativeTryServiceAuthentication(ctx context.Context, operation *nativeAuthOperation,
	request AuthenticateRequest,
) (bool, error) {
	if request.Service == nil || !sdk.nativeOneFactorEligible(operation.state, *request.Service) {
		return false, nil
	}

	return sdk.nativeOneFactor(ctx, operation, request)
}

func (sdk *SDK) nativeTryTokenAuthentication(ctx context.Context, operation *nativeAuthOperation,
	request AuthenticateRequest,
) (bool, error) {
	if !nativeHasToken(operation.state) {
		return false, nil
	}

	accepted, err := sdk.nativeLoginToken(ctx, operation, !request.PauseTwoFactor, request.AcceptTerms)
	if err != nil && !nativeCanRetry(err) {
		return false, err
	}

	return accepted, nil
}
