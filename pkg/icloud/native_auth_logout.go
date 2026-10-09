package icloud

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/portpowered/go-icloud/internal/authapi"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

// Logout attempts remote logout and returns cleared caller-owned credentials by default.
// A refused remote logout remains inspectable through the ordered response metadata.
func (sdk *SDK) Logout(ctx context.Context, input LogoutRequest) (*LogoutResult, error) {
	operation, err := newNativeAuthOperation(ctx, "Logout", input.Auth, input.State)
	if err != nil {
		return nil, err
	}

	confirmed := false
	if operation.state.Auth.AccountID != "" && hasWebAuthCookie(operation.state.Auth) {
		confirmed, err = sdk.nativeRemoteLogout(ctx, operation, input)
		if err != nil {
			return nil, err
		}
	}

	if !input.PreserveLocalSession {
		operation.state = clearedNativeAuthState(operation.state)
	}

	return &LogoutResult{State: cloneNativeAuthState(operation.state), LocalCleared: !input.PreserveLocalSession,
		RemoteConfirmed: confirmed, Responses: cloneDriveResponses(operation.responses)}, nil
}

func (sdk *SDK) nativeRemoteLogout(ctx context.Context, operation *nativeAuthOperation,
	input LogoutRequest,
) (bool, error) {
	payload := auth.AuthLogoutRequest{TrustBrowser: input.KeepTrusted, AllBrowsers: input.AllSessions}
	params := authapi.LogoutAuthSessionParams(nativeAuthParams(operation.state))

	request, err := nativeEncodedRequest(payload, func(body io.Reader) (*http.Request, error) {
		return nativeGeneratedRequest(authapi.NewLogoutAuthSessionRequestWithBody(
			operation.state.Auth.SetupServiceURL, &params,
			protocol.AuthLogoutContentTypeValue, body))
	})
	if err != nil {
		return false, newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	headers := requestHeaders(operation.state.Auth.Headers)
	headers.Set(protocol.AuthHTTPContentTypeName, protocol.AuthLogoutContentTypeValue)

	response, err := sdk.nativeAuthExchange(ctx, operation, request, headers)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false, err
		}

		return false, nil
	}

	err = ctx.Err()
	if err != nil {
		return false, driveContextFailure(operation.name, err)
	}

	if nativeRequireSuccess(operation, response) != nil {
		//nolint:nilerr // Source clears local state even when the provider refuses remote logout.
		return false, nil
	}

	var verdict auth.AuthSuccessResponse
	if nativeDecodeAuthObject(response.Body, &verdict) != nil {
		//nolint:nilerr // Source catches JSON ValueError and returns unconfirmed remote logout.
		return false, nil
	}

	return authTrue(verdict.Success), nil
}

func clearedNativeAuthState(state NativeAuthState) NativeAuthState {
	boundary := cloneDriveAuth(state.Auth)
	boundary.AccountID = ""
	boundary.SessionToken = nil
	boundary.Cookies = []AuthCookie{}
	boundary.PhotosServiceURL = ""
	boundary.PhotosUploadServiceURL = ""
	boundary.SharedPhotosServiceURL = ""
	boundary.DriveServiceURL = ""
	boundary.FindMyServiceURL = ""
	boundary.RemindersServiceURL = ""
	boundary.LegacyRemindersServiceURL = ""
	boundary.AccountServiceURL = ""
	boundary.DriveDocumentServiceURL = ""
	boundary.Headers = nil

	return initialNativeAuthState(AuthenticateRequest{Auth: boundary, AccountName: state.AccountName,
		TrustToken: "", AccountCountryCode: nil, Password: "", SavedState: nil, Service: nil,
		AcceptTerms: false, ForceRefresh: false, PauseTwoFactor: false})
}
