package icloud

import (
	"bytes"
	"context"
	"maps"
)

// ApplySessionResponses copies a native session and applies response cookie/token rotations without network I/O.
// Account discovery and trust flags remain those of the saved authentication result.
func (sdk *SDK) ApplySessionResponses(ctx context.Context,
	request ApplySessionResponsesRequest,
) (*ApplySessionResponsesResult, error) {
	err := ctx.Err()
	if err != nil {
		return nil, newClientError("ApplySessionResponses", Canceled, 0, nil, nil, err)
	}

	state := authResumeState{auth: cloneDriveAuth(request.Session.Auth), trustToken: request.Session.TrustToken,
		responses: cloneDriveResponses(request.Session.Responses), response: nil, refreshed: false,
		country: maps.Clone(request.Session.AccountCountryCode)}
	for _, metadata := range cloneDriveResponses(request.Responses) {
		state.recordMetadata(metadata)
	}

	result := request.Session
	result.Auth = state.auth
	result.TrustToken = state.trustToken
	result.AccountCountryCode = state.country
	result.Responses = state.responses
	result.AccountData = bytes.Clone(request.Session.AccountData)

	return &ApplySessionResponsesResult{Session: result}, nil
}
