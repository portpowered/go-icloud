package webtransport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/authapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

// AuthResponse binds decoded discovery to its exact HTTP evidence.
type AuthResponse struct {
	Data     auth.AuthAccountResponse
	Response *BytesResponse
}

// ValidateAuthSession validates web cookies without account query parameters.
func (client *Client) ValidateAuthSession(ctx context.Context, origin string, headers http.Header,
	cookies *CookieState,
) (*AuthResponse, error) {
	body, err := referenceJSON(nil)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	request, err := authapi.NewValidateAuthSessionRequestWithBody(origin, nil, "", bytes.NewReader(body))
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	return client.readAuth(ctx, origin, headers, cookies, request)
}

// LoginAuthToken refreshes caller-owned tokens without a password.
func (client *Client) LoginAuthToken(ctx context.Context, origin string, headers http.Header,
	cookies *CookieState, input auth.AuthTokenLoginRequest,
) (*AuthResponse, error) {
	body, err := referenceJSON(input)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	request, err := authapi.NewLoginAuthTokenRequestWithBody(origin, nil, jsonMedia(),
		bytes.NewReader(body))
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	return client.readAuth(ctx, origin, headers, cookies, request)
}

// LoginAuthCredentials submits the provider's verified service-specific one-factor request.
func (client *Client) LoginAuthCredentials(ctx context.Context, origin string, headers http.Header,
	cookies *CookieState, input auth.AuthCredentialsLoginRequest,
) (*AuthResponse, error) {
	body, err := referenceJSON(input)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	request, err := authapi.NewLoginAuthTokenRequestWithBody(origin, nil, jsonMedia(),
		bytes.NewReader(body))
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	return client.readAuth(ctx, origin, headers, cookies, request)
}

func (client *Client) readAuth(ctx context.Context, origin string, headers http.Header,
	cookies *CookieState, request *http.Request,
) (*AuthResponse, error) {
	err := validateOrigin(origin)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	contentType := request.Header.Get(protocol.HTTPContentTypeName)
	request = request.WithContext(ctx)

	request.Header = callerHeaders(headers)

	if request.Header.Get(protocol.HTTPContentTypeName) == "" && contentType != "" {
		request.Header.Set(protocol.HTTPContentTypeName, contentType)
	}

	if request.Header.Get(protocol.AcceptName) == "" {
		request.Header.Set(protocol.AcceptName, jsonMedia())
	}

	response, err := client.readPrepared(request, successfulContent, cookies)
	if err != nil {
		return nil, err
	}

	_, err = accountFields(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	var data auth.AuthAccountResponse

	err = json.Unmarshal(response.Body, &data)
	if err != nil {
		return nil, responseFailure(Decode, fmt.Errorf("decode authentication discovery: %w", err), response)
	}

	return &AuthResponse{Data: data, Response: response}, nil
}
