package icloud

import (
	"context"
	"encoding/json"

	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/authapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

func (sdk *SDK) nativeAcceptTerms(ctx context.Context, operation *nativeAuthOperation,
	login auth.AuthTokenLoginRequest, account auth.AuthAccountResponse,
) (bool, error) {
	return sdk.nativeAcceptTermsLogin(ctx, operation, webtransport.LoginAuthTokenCall{
		Origin: operation.state.Auth.SetupServiceURL, Params: nil, Body: login}, account)
}

func (sdk *SDK) nativeAcceptTermsLogin(ctx context.Context, operation *nativeAuthOperation,
	login webtransport.AuthenticationCall, account auth.AuthAccountResponse,
) (bool, error) {
	locale := protocol.AuthDefaultLocaleValue
	if account.DsInfo != nil && account.DsInfo.LanguageCode != nil {
		locale = *account.DsInfo.LanguageCode
	}

	params := authapi.GetAuthTermsParams(nativeAuthParams(operation.state))

	input := auth.AuthGetTermsRequest{Locale: locale}

	request, err := nativeEncodedRequest(webtransport.GetAuthTermsCall{
		Origin: operation.state.Auth.SetupServiceURL, Params: &params, Body: input,
	})
	if err != nil {
		return false, newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, request, nativeSetupJSONHeaders(operation.state))
	if err != nil {
		return false, err
	}

	err = nativeRequireSuccess(operation, response)
	if err != nil {
		return false, err
	}

	var terms auth.AuthTermsResponse

	err = nativeDecodeAuthObject(response.Body, &terms)
	if err != nil {
		return false, nativeResponseError(operation, response, err, InvalidResponse)
	}

	if terms.ICloudTerms == nil || terms.ICloudTerms.Version == nil {
		return false, nativeResponseError(operation, response, errUpdatedTerms, TermsRequired)
	}

	err = sdk.nativeRepairTerms(ctx, operation, *terms.ICloudTerms.Version)
	if err != nil {
		return false, err
	}

	return sdk.nativeTermsRelogin(ctx, operation, login)
}

func (sdk *SDK) nativeRepairTerms(ctx context.Context, operation *nativeAuthOperation, version int) error {
	params := authapi.AcceptAuthTermsParams(nativeAuthParams(operation.state))

	request, err := nativeEncodedRequest(webtransport.AcceptAuthTermsCall{
		Origin: operation.state.Auth.SetupServiceURL, Params: &params, Body: auth.AuthAcceptTermsRequest{AcceptedICloudTerms: version},
	})
	if err != nil {
		return newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, request, nativeSetupJSONHeaders(operation.state))
	if err != nil {
		return err
	}

	return nativeRequireSuccess(operation, response)
}

func (sdk *SDK) nativeTermsRelogin(ctx context.Context, operation *nativeAuthOperation,
	login webtransport.AuthenticationCall,
) (bool, error) {
	request, err := nativeEncodedRequest(login)
	if err != nil {
		return false, newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, request, nativeSetupJSONHeaders(operation.state))
	if err != nil {
		return false, err
	}

	err = nativeRequireSuccess(operation, response)
	if err != nil {
		return false, err
	}

	var account auth.AuthAccountResponse

	err = nativeDecodeAuthObject(response.Body, &account)
	if err != nil {
		return false, nativeResponseError(operation, response, err, InvalidResponse)
	}

	sdk.nativeAccountDiscovery(operation, &webtransport.AuthResponse{Data: account, Response: response})

	return true, nil
}

func (sdk *SDK) nativeOneFactor(ctx context.Context, operation *nativeAuthOperation,
	input AuthenticateRequest,
) (bool, error) {
	login := nativeOneFactorPayload(operation.state, input)

	response, err := sdk.nativeOneFactorLogin(ctx, operation, login)
	if err != nil {
		if nativeCanRetry(err) {
			return false, nil
		}

		return false, err
	}

	if !hasWebAuthCookie(operation.state.Auth) {
		return false, nil
	}

	err = sdk.nativeOneFactorTerms(ctx, operation, input, login, response)
	if err != nil {
		return false, err
	}

	return sdk.nativeOneFactorValidate(ctx, operation)
}

func nativeOneFactorPayload(state NativeAuthState, input AuthenticateRequest) auth.AuthCredentialsLoginRequest {
	service := nullable.Nullable[string]{}
	if input.Service == nil {
		service.SetNull()
	} else {
		service.Set(*input.Service)
	}

	return auth.AuthCredentialsLoginRequest{AppName: service, AppleId: state.AccountName, Password: input.Password}
}

func (sdk *SDK) nativeOneFactorTerms(ctx context.Context, operation *nativeAuthOperation,
	input AuthenticateRequest, login auth.AuthCredentialsLoginRequest, response *webtransport.BytesResponse,
) error {
	var account auth.AuthAccountResponse
	if len(operation.state.AccountData) > 0 {
		err := json.Unmarshal(operation.state.AccountData, &account)
		if err != nil {
			return nativeResponseError(operation, response, err, InvalidResponse)
		}
	}

	if !authTrue(account.TermsUpdateNeeded) {
		return nil
	}

	if !input.AcceptTerms {
		return nativeResponseError(operation, response, errUpdatedTerms, TermsRequired)
	}

	_, err := sdk.nativeAcceptTermsLogin(ctx, operation, webtransport.LoginAuthCredentialsCall{
		Origin: operation.state.Auth.SetupServiceURL, Params: nil, Body: login}, account)

	return err
}

func (sdk *SDK) nativeOneFactorValidate(ctx context.Context, operation *nativeAuthOperation) (bool, error) {
	validated, err := sdk.web.ValidateAuthSession(ctx, operation.state.Auth.SetupServiceURL,
		requestHeaders(operation.state.Auth.Headers), operation.cookies)
	if err != nil {
		failure := nativeRecordFailure(operation, err, false)
		if nativeCanRetry(failure) {
			return false, nil
		}

		return false, failure
	}

	operation.record(validated.Response)
	sdk.nativeAccountDiscovery(operation, validated)

	return true, nil
}

func (sdk *SDK) nativeOneFactorLogin(ctx context.Context, operation *nativeAuthOperation,
	login auth.AuthCredentialsLoginRequest,
) (*webtransport.BytesResponse, error) {
	request, err := nativeEncodedRequest(webtransport.LoginAuthCredentialsCall{
		Origin: operation.state.Auth.SetupServiceURL, Params: nil, Body: login,
	})
	if err != nil {
		return nil, newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, request, nativeSetupJSONHeaders(operation.state))
	if err != nil {
		return nil, err
	}

	err = nativeRequireSuccess(operation, response)
	if err != nil {
		return nil, err
	}

	return response, nil
}
