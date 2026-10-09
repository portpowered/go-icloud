package icloud

import (
	"context"

	"github.com/portpowered/go-icloud/internal/authapi"
	"github.com/portpowered/go-icloud/internal/protocol"
)

// TrustSession obtains provider trust and refreshed account discovery using copied credentials.
func (sdk *SDK) TrustSession(ctx context.Context, request NativeAuthRequest) (*NativeAuthResult, error) {
	operation, err := newNativeAuthOperation(ctx, "TrustSession", request.Auth, request.State)
	if err != nil {
		return nil, err
	}

	err = sdk.nativeTrust(ctx, operation)

	if err != nil {
		return nil, err
	}

	return nativeAuthResult(operation)
}

func (sdk *SDK) nativeTrust(ctx context.Context, operation *nativeAuthOperation) error {
	operation.state.RequiresMFA = false

	request, err := authapi.NewTrustAuthSessionRequest(nativeIDMSOrigin(operation.state))

	if err != nil {
		return newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, request, nativeAuthHeaders(operation.state, protocol.AuthAcceptValue))
	if err != nil {
		if nativeCanRetry(err) {
			operation.success = false
			return nil
		}

		return err
	}

	err = nativeRequireSuccess(operation, response)

	if err != nil {
		operation.success = false

		return nil
	}

	accepted, err := sdk.nativeLoginToken(ctx, operation, true, false)
	if err != nil && !nativeCanRetry(err) {
		return err
	}

	operation.success = accepted

	return nil
}
