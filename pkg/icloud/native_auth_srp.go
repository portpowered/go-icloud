package icloud

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/portpowered/go-icloud/internal/authapi"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/srp"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
	srpmodels "github.com/portpowered/go-icloud/pkg/dependencymodels/srp"
)

func (sdk *SDK) nativePassword(ctx context.Context, operation *nativeAuthOperation,
	input AuthenticateRequest,
) error {
	if err := sdk.nativeAuthorize(ctx, operation); err != nil {
		return err
	}

	user, err := srp.New(operation.state.AccountName, input.Password, sdk.random)
	if err != nil {
		failure := newClientError(operation.name, Configuration, 0, nil, nil, err)
		failure.prior = cloneDriveResponses(operation.responses)
		return failure
	}

	challenge, err := sdk.nativeSRPChallenge(ctx, operation, user.Public())
	if err != nil {
		return err
	}

	proof, err := user.Challenge(challenge.Salt, challenge.B, challenge.Iteration,
		srpmodels.Protocol(challenge.Protocol))
	if err != nil {
		return nativeResponseError(operation, operation.response, err, InvalidResponse)
	}

	err = sdk.nativeSRPComplete(ctx, operation, input, challenge.C, proof)

	if err != nil {
		return err
	}

	if operation.state.RequiresMFA || operation.passwordTokenLogged {
		return nil
	}

	_, err = sdk.nativeLoginToken(ctx, operation, !input.PauseTwoFactor, input.AcceptTerms)

	return err
}

func (sdk *SDK) nativeAuthorize(ctx context.Context, operation *nativeAuthOperation) error {
	state := operation.state
	widget, version, latest := protocol.AuthOAuthClientIDValue, protocol.AuthSkVersionValue, protocol.AuthVersionValue
	responseType, mode, home := protocol.AuthOAuthResponseTypeValue, protocol.AuthOAuthResponseModeValue, nativeHomeOrigin(state)
	params := authapi.AuthorizeAuthSignInParams{FrameId: &state.Auth.ClientID, SkVersion: &version,
		Iframeid: &state.Auth.ClientID, ClientId: &widget, ResponseType: &responseType, RedirectUri: &home,
		ResponseMode: &mode, State: &state.Auth.ClientID, AuthVersion: &latest}

	request, err := authapi.NewAuthorizeAuthSignInRequest(nativeIDMSOrigin(state), &params)
	if err != nil {
		return newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, request, requestHeaders(state.Auth.Headers))
	if err != nil {
		return err
	}

	return nativeRequireSuccess(operation, response)
}

func (sdk *SDK) nativeSRPChallenge(ctx context.Context, operation *nativeAuthOperation,
	public []byte,
) (*auth.AuthSRPInitResponse, error) {
	input := auth.AuthSRPInitRequest{A: public, AccountName: operation.state.AccountName,
		Protocols: []auth.AuthSRPProtocol{auth.S2k, auth.S2kFo}}

	request, err := nativeEncodedRequest(input, func(body io.Reader) (*http.Request, error) {
		return authapi.NewInitAuthSRPRequestWithBody(nativeIDMSOrigin(operation.state),
			protocol.AuthMediaApplicationJson, body)
	})
	if err != nil {
		return nil, newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, request, nativeAuthHeaders(operation.state, protocol.AuthAcceptValue))
	if err != nil {
		return nil, err
	}

	err = nativeRequireSuccess(operation, response)

	if err != nil {
		return nil, err
	}

	var challenge auth.AuthSRPInitResponse
	err = json.Unmarshal(response.Body, &challenge)
	if err != nil {
		return nil, nativeResponseError(operation, response, err, InvalidResponse)
	}
	if challenge.C == "" || len(challenge.Salt) == 0 || len(challenge.B) == 0 {
		return nil, nativeResponseError(operation, response, errNativeAuthInput, InvalidResponse)
	}

	return &challenge, nil
}

func (sdk *SDK) nativeSRPComplete(ctx context.Context, operation *nativeAuthOperation,
	input AuthenticateRequest, challenge string, proof srp.Proof,
) error {
	data := auth.AuthSRPCompleteRequest{AccountName: operation.state.AccountName, C: challenge,
		M1: proof.M1, M2: proof.M2, RememberMe: true, TrustTokens: []string{}, Pause2FA: nil}
	if operation.state.TrustToken != "" {
		data.TrustTokens = append(data.TrustTokens, operation.state.TrustToken)
	}

	if input.PauseTwoFactor {
		pause := auth.AuthSRPCompleteRequestPause2FA(true)
		data.Pause2FA = &pause
	}

	remember := protocol.AuthRememberMeQueryValue
	params := authapi.CompleteAuthSRPParams{IsRememberMeEnabled: &remember}

	request, err := nativeEncodedRequest(data, func(body io.Reader) (*http.Request, error) {
		return authapi.NewCompleteAuthSRPRequestWithBody(nativeIDMSOrigin(operation.state), &params,
			protocol.AuthMediaApplicationJson, body)
	})
	if err != nil {
		return newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, request, nativeAuthHeaders(operation.state, protocol.AuthAcceptValue))
	if err != nil {
		return err
	}

	if nativeLockedBody(response.Body) {
		return nativeResponseError(operation, response, errNativeAuthInput, AccountLocked)
	}
	if response.Status != http.StatusConflict {
		return nativeRequireSuccess(operation, response)
	}

	if input.PauseTwoFactor && nativeHasToken(operation.state) {
		accepted, tokenErr := sdk.nativeLoginToken(ctx, operation, false, input.AcceptTerms)
		if tokenErr != nil && !nativeCanRetry(tokenErr) {
			return tokenErr
		}

		if accepted {
			operation.passwordTokenLogged = true
			return nil
		}
	}

	operation.state.RequiresMFA = true
	err = sdk.nativeGetChallenge(ctx, operation)
	if err != nil {
		return err
	}

	err = sdk.nativeRequestCode(ctx, operation, nil)
	if nativeCanRetry(err) {
		return nil
	}

	return err
}
