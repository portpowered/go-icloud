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
	if err := ctx.Err(); err != nil {
		return nil, driveContextFailure(name, err)
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

	if !request.ForceRefresh && nativeHasToken(operation.state) {
		reused, reuseErr := sdk.nativeReuseCookie(ctx, operation, request.PauseTwoFactor)
		if reuseErr != nil {
			return nil, reuseErr
		}

		if reused {
			return nativeAuthResult(operation)
		}
	}

	if request.Service != nil && sdk.nativeOneFactorEligible(operation.state, *request.Service) {
		accepted, serviceErr := sdk.nativeOneFactor(ctx, operation, request)
		if serviceErr != nil {
			return nil, serviceErr
		}

		if accepted {
			return nativeAuthResult(operation)
		}
	}

	if nativeHasToken(operation.state) {
		accepted, tokenErr := sdk.nativeLoginToken(ctx, operation, !request.PauseTwoFactor, request.AcceptTerms)
		if tokenErr != nil && !nativeCanRetry(tokenErr) {
			return nil, tokenErr
		}

		if accepted {
			return nativeAuthResult(operation)
		}
	}

	if request.Password == "" {
		return nil, newClientError(name, Unauthorized, 0, nil, nil, errNativeAuthInput)
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
		AccountData: []byte("{}"), Challenge: NativeAuthChallenge{Mode: "", PhoneNumbers: []TrustedPhoneNumber{},
			SecurityKeyNames: []string{}, AuthInitialRoute: "", HasTrustedDevices: false, AuthFactors: []string{},
			BridgeBootstrap: nil, SecurityKeyChallenge: nil}, DeliveryMethod: TwoFactorDeliveryUnknown,
		CodeRequested: false, RequiresMFA: false}
	if request.SavedState != nil {
		state = cloneNativeAuthState(*request.SavedState)
		state.Auth = cloneDriveAuth(request.Auth)
	}

	state.AccountName = request.AccountName
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
