package icloud

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/portpowered/go-icloud/internal/webtransport"
)

type nativeRequestBuilder func(io.Reader) (*http.Request, error)

func nativeEncodedRequest(input any, builder nativeRequestBuilder) (*http.Request, error) {
	body, err := webtransport.EncodeAuthenticationRequest(input)
	if err != nil {
		return nil, fmt.Errorf("prepare authentication body: %w", err)
	}

	request, err := builder(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("prepare authentication request: %w", err)
	}

	return request, nil
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
