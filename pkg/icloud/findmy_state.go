package icloud

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/findmy"
)

// Snapshot returns a copied cache without performing network I/O, including after Close.
func (session *FindMySession) Snapshot() (*FindMySnapshot, error) {
	session.mu.Lock()

	devices := make([]findmy.FindMyDevice, 0, len(session.order))
	for _, id := range session.order {
		devices = append(devices, session.devices[id])
	}

	user := session.user
	server := append(json.RawMessage(nil), session.server...)
	session.mu.Unlock()

	result := &FindMySnapshot{Devices: make([]FindMyDevice, 0), UserInfo: nullable.NewNullNullable[FindMyUserInfo](),
		ServerContext: nullable.NewNullNullable[json.RawMessage]()}
	if len(server) != 0 && string(server) != "null" {
		result.ServerContext = nullable.NewNullableWithValue(server)
	}

	err := projectFindMyValue(devices, &result.Devices)
	if err == nil && user.IsSpecified() && !user.IsNull() {
		var body []byte

		body, err = json.Marshal(user.MustGet())
		if err == nil && string(body) != "{}" {
			err = projectFindMyValue(user, &result.UserInfo)
		}
	}

	if err != nil {
		return nil, newClientError("FindMySnapshot", InvalidResponse, 0, nil, nil, err)
	}

	return result, nil
}

func projectFindMyValue(input, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("encode Find My projection: %w", err)
	}

	err = json.Unmarshal(body, output)
	if err != nil {
		return fmt.Errorf("decode Find My projection: %w", err)
	}

	return nil
}

func (session *FindMySession) bindCookies() {
	session.auth = bindSessionCookies(session.auth, session.auth.FindMyServiceURL)
}

func (session *FindMySession) recordResponse(metadata ResponseMetadata) {
	auth := applySessionResponse(session.Authentication(), metadata)
	session.mu.Lock()
	session.auth = auth
	session.responses = append(session.responses, metadata)
	session.mu.Unlock()
}

func (session *FindMySession) clearResponses() {
	session.mu.Lock()
	session.responses = nil
	session.mu.Unlock()
}

func (session *FindMySession) clearLastError() {
	session.mu.Lock()
	session.lastError = nil
	session.mu.Unlock()
}

func (session *FindMySession) recordFailure(operation string, err error) *ClientError {
	var failure *ClientError
	if !errors.As(err, &failure) {
		failure = newClientError(operation, Transport, 0, nil, nil, err)
	}

	failure.prior = session.LastResponses()
	if failure.StatusCode() != 0 {
		session.recordResponse(ResponseMetadata{CookieScopeURL: failure.CookieScopeURL(),
			StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders()})
	}

	session.mu.Lock()
	session.lastError = failure
	session.mu.Unlock()

	return failure
}

func adaptFindMyFailure(operation string, err error) *ClientError {
	var failure *ClientError
	if errors.As(err, &failure) {
		return failure
	}

	return adaptFailure(operation, err)
}

// LastError returns a copied failure from the latest refresh/monitor operation, or nil after success.
func (session *FindMySession) LastError() *ClientError {
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.lastError == nil {
		return nil
	}

	result := *session.lastError
	result.body = session.lastError.ResponseBody()
	result.headers = session.lastError.ResponseHeaders()
	result.prior = session.lastError.PriorResponses()

	return &result
}
