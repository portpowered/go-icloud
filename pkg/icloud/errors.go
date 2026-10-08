package icloud

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/portpowered/go-icloud/internal/webtransport"
)

// ClientError is an inspectable failure with a safe display message.
// Use errors.As to inspect it and errors.Is to inspect an underlying cause.
type ClientError struct {
	cookieScopeURL string
	prior          []ResponseMetadata
	operation      string
	kind           ErrorKind
	status         int
	body           []byte
	headers        []Header
	cause          error
}

// Error never prints credentials, raw response content or an underlying transport message.
func (failure *ClientError) Error() string {
	return fmt.Sprintf("icloud %s failed: %s (status %d)", failure.operation, failure.kind, failure.status)
}

// Unwrap retains the original cause for explicit caller inspection.
func (failure *ClientError) Unwrap() error { return failure.cause }

// Kind returns the failure classification.
func (failure *ClientError) Kind() ErrorKind { return failure.kind }

// StatusCode returns the provider status, or zero when no response arrived.
func (failure *ClientError) StatusCode() int { return failure.status }

// ResponseBody returns a copy of exact provider bytes, which may contain private data.
func (failure *ClientError) ResponseBody() []byte { return append([]byte(nil), failure.body...) }

// ResponseHeaders returns a copy of response headers, including session update values.
func (failure *ClientError) ResponseHeaders() []Header {
	return append([]Header(nil), failure.headers...)
}

// PriorResponses returns copied metadata from earlier completed exchanges in a failed operation.
// Authentication updates from these responses remain available even if a later request fails.
func (failure *ClientError) PriorResponses() []ResponseMetadata {
	result := make([]ResponseMetadata, 0, len(failure.prior))
	for _, metadata := range failure.prior {
		result = append(result, ResponseMetadata{CookieScopeURL: metadata.CookieScopeURL, StatusCode: metadata.StatusCode,
			Headers: append([]Header(nil), metadata.Headers...)})
	}

	return result
}

func newClientError(operation string, kind ErrorKind, status int, body []byte,
	headers []Header, cause error,
) *ClientError {
	return &ClientError{
		prior:          nil,
		cookieScopeURL: "",
		operation:      operation, kind: kind, status: status,
		body: append([]byte(nil), body...), headers: append([]Header(nil), headers...), cause: cause,
	}
}

func adaptFailure(operation string, err error) *ClientError {
	var failure *webtransport.ResponseError

	if !errors.As(err, &failure) {
		return newClientError(operation, Transport, 0, nil, nil, err)
	}

	kind := transportKind(failure)

	var timeoutError net.Error

	if errors.As(err, &timeoutError) && timeoutError.Timeout() {
		kind = Timeout
	}

	if errors.Is(err, context.Canceled) {
		kind = Canceled
	}

	if errors.Is(err, context.DeadlineExceeded) {
		kind = Timeout
	}

	result := newClientError(operation, kind, failure.Status, failure.Body, responseHeaders(failure.Headers), err)
	result.cookieScopeURL = failure.CookieScopeURL

	for _, prior := range failure.Prior {
		result.prior = append(result.prior, publicMetadata(prior))
	}

	return result
}

func transportKind(failure *webtransport.ResponseError) ErrorKind {
	switch failure.Stage {
	case webtransport.Configuration:
		return Configuration
	case webtransport.Decode:
		return InvalidResponse
	case webtransport.Provider:
		return providerKind(failure.Status)
	case webtransport.Transport:
		return Transport
	default:
		return Transport
	}
}

func providerKind(status int) ErrorKind {
	switch status {
	case http.StatusUnauthorized:
		return Unauthorized
	case http.StatusForbidden:
		return Forbidden
	case http.StatusNotFound:
		return NotFound
	case http.StatusTooManyRequests:
		return RateLimited
	case http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusGatewayTimeout:
		return Unavailable
	default:
		return Provider
	}
}

// CookieScopeURL returns the response request origin/path for applying Set-Cookie.
// Query credentials are excluded; an empty value means no response arrived.
func (failure *ClientError) CookieScopeURL() string { return failure.cookieScopeURL }
