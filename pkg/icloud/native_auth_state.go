package icloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/internal/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

var errNativeAuthInput = errors.New("native authentication state or input is incomplete")

type nativeAuthOperation struct {
	name                string
	state               NativeAuthState
	responses           []ResponseMetadata
	cookies             *webtransport.CookieState
	success             bool
	response            *webtransport.BytesResponse
	passwordTokenLogged bool
}

func newNativeAuthOperation(ctx context.Context, name string, boundary AuthContext,
	state NativeAuthState,
) (*nativeAuthOperation, error) {
	contextErr := ctx.Err()
	if contextErr != nil {
		return nil, driveContextFailure(name, contextErr)
	}

	state = cloneNativeAuthState(state)

	state.Auth = cloneDriveAuth(boundary)

	if state.Auth.ClientID == "" {
		return nil, newClientError(name, Configuration, 0, nil, nil, errNativeAuthInput)
	}

	cookies, err := webtransport.NewCookieState(authCookies(state.Auth.Cookies))
	if err != nil {
		return nil, newClientError(name, Configuration, 0, nil, nil, err)
	}

	return &nativeAuthOperation{name: name, state: state, responses: []ResponseMetadata{}, cookies: cookies, success: true,
		response: nil, passwordTokenLogged: false}, nil
}

func cloneNativeAuthState(state NativeAuthState) NativeAuthState {
	state.DeliveryNotice = copyString(state.DeliveryNotice)
	state.Auth = cloneDriveAuth(state.Auth)
	state.AccountCountryCode = maps.Clone(state.AccountCountryCode)
	state.AccountData = bytes.Clone(state.AccountData)

	state.Challenge.PhoneNumbers = slices.Clone(state.Challenge.PhoneNumbers)

	for index := range state.Challenge.PhoneNumbers {
		phone := &state.Challenge.PhoneNumbers[index]

		phone.ID.union = bytes.Clone(phone.ID.union)
		if phone.NonFTEU != nil {
			value := *phone.NonFTEU
			phone.NonFTEU = &value
		}
	}

	state.Challenge.AuthFactors = slices.Clone(state.Challenge.AuthFactors)
	state.Challenge.SecurityKeyNames = slices.Clone(state.Challenge.SecurityKeyNames)

	state.Challenge.ProviderData = bytes.Clone(state.Challenge.ProviderData)
	state.Challenge.BridgeBootstrap = bytes.Clone(state.Challenge.BridgeBootstrap)

	if state.Challenge.SecurityKeyChallenge != nil {
		challenge := *state.Challenge.SecurityKeyChallenge
		challenge.CredentialIDs = slices.Clone(challenge.CredentialIDs)
		state.Challenge.SecurityKeyChallenge = &challenge
	}

	return state
}

func (operation *nativeAuthOperation) record(response *webtransport.BytesResponse) {
	operation.response = response
	metadata := publicMetadata(response)
	state := authResumeState{auth: operation.state.Auth, trustToken: operation.state.TrustToken,
		country: operation.state.AccountCountryCode, responses: nil, response: nil, refreshed: false}
	state.recordMetadata(metadata)
	operation.state.Auth, operation.state.TrustToken = state.auth, state.trustToken
	operation.state.AccountCountryCode = state.country
	operation.recordAuthHeaders(response.Headers)
	operation.responses = append(operation.responses, metadata)
}

func (operation *nativeAuthOperation) recordAuthHeaders(headers http.Header) {
	values := requestHeaders(operation.state.Auth.Headers)

	for _, name := range []string{protocol.AuthHTTPScntName, protocol.AuthHTTPXAppleIDSessionIdName,
		protocol.AuthHTTPXAppleAuthAttributesName} {
		if value := headers.Get(name); value != "" {
			values.Set(name, value)
		}
	}

	operation.state.Auth.Headers = responseHeaders(values)
}

func (operation *nativeAuthOperation) failure(err error) *ClientError {
	failure := adaptFailure(operation.name, err)
	failure.prior = cloneDriveResponses(operation.responses)

	return failure
}

func (sdk *SDK) nativeAuthExchange(ctx context.Context, operation *nativeAuthOperation,
	request *http.Request, headers http.Header,
) (*webtransport.BytesResponse, error) {
	response, err := sdk.web.ExchangeAuthentication(ctx, request, headers, operation.cookies)
	if err != nil {
		return nil, nativeRecordFailure(operation, err, false)
	}

	operation.record(response)

	return response, nil
}

func nativeAuthResult(operation *nativeAuthOperation) (*NativeAuthResult, error) {
	var account auth.AuthAccountResponse
	if len(operation.state.AccountData) != 0 {
		err := json.Unmarshal(operation.state.AccountData, &account)
		if err != nil {
			return nil, newClientError(operation.name, InvalidResponse, 0, nil, nil, err)
		}
	}

	trusted := authTrue(account.HsaTrustedBrowser)
	required := authTrue(account.HsaChallengeRequired) || !trusted || operation.state.RequiresMFA

	version := 0
	if account.DsInfo != nil && account.DsInfo.HsaVersion != nil {
		version = *account.DsInfo.HsaVersion
	}

	return &NativeAuthResult{State: cloneNativeAuthState(operation.state), TrustedSession: trusted,
		Success:           operation.success,
		RequiresTwoFactor: nativeTwoFactorRequired(operation.state, version, required),
		RequiresTwoStep:   required && version >= 1,
		Responses:         cloneDriveResponses(operation.responses)}, nil
}

func nativeIDMSOrigin(_ NativeAuthState) string {
	return protocol.AuthIDMSOriginValue
}

func nativeHomeOrigin(state NativeAuthState) string {
	if state.Auth.ChinaMainland != nil && *state.Auth.ChinaMainland {
		return protocol.AuthHomeChinaOriginValue
	}

	return protocol.AuthHomeOriginValue
}

func nativeTwoFactorRequired(state NativeAuthState, version int, required bool) bool {
	return required && (version == 2 || state.RequiresMFA || len(state.Challenge.ProviderData) > 0)
}
