package icloud

import (
	"net/http"

	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
)

func nativeEncodedRequest(call webtransport.AuthenticationCall) (webtransport.AuthenticationCall, error) {
	err := webtransport.ValidateAuthenticationCall(call)
	if err != nil {
		return nil, err
	}

	return call, nil
}

func nativeResponseError(operation *nativeAuthOperation, response *webtransport.BytesResponse,
	cause error, kind ErrorKind,
) *ClientError {
	failure := newClientError(operation.name, kind, response.Status, response.Body,
		responseHeaders(response.Headers), cause)

	failure.cookieScopeURL = response.CookieScopeURL

	if len(operation.responses) > 0 {
		failure.prior = cloneDriveResponses(operation.responses[:len(operation.responses)-1])
	}

	return failure
}

func nativeRequireSuccess(operation *nativeAuthOperation, response *webtransport.BytesResponse) error {
	if nativeLockedBody(response.Body) {
		return nativeResponseError(operation, response, errNativeAuthInput, AccountLocked)
	}

	if response.Status < http.StatusOK || response.Status >= http.StatusMultipleChoices {
		return nativeResponseError(operation, response, errNativeAuthInput, providerKind(response.Status))
	}

	return nil
}
