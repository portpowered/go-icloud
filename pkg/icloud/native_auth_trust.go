package icloud

import (
	"context"
	"errors"
	"net/http"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
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

	request, err := nativeEncodedRequest(webtransport.TrustAuthSessionCall{
		Origin: nativeIDMSOrigin(operation.state), Params: nil,
	})
	if err != nil {
		return newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, request,
		nativeAuthHeaders(operation.state, protocol.AuthAcceptValue))
	if err != nil {
		if nativeTrustRefused(err) {
			operation.success = false

			return nil
		}

		return err
	}

	if !nativeTrustAccepted(response) {
		operation.success = false

		return nil
	}

	accepted, err := sdk.nativeLoginToken(ctx, operation, true, operation.state.AcceptTerms)
	if err != nil && !nativeCanRetry(err) {
		return err
	}

	operation.success = accepted

	return nil
}

func nativeTrustRefused(err error) bool {
	if nativeCanRetry(err) {
		return true
	}

	var failure *ClientError

	// Source's successful-status JSON dispatcher treats a reason-bearing body as
	// an API refusal. Its locked-account exception is raised only for non-OK HTTP
	// responses; preserve that distinction after shared failure classification.
	return errors.As(err, &failure) && failure.Kind() == AccountLocked &&
		failure.StatusCode() >= http.StatusOK && failure.StatusCode() < http.StatusMultipleChoices
}

func nativeTrustAccepted(response *webtransport.BytesResponse) bool {
	return response.Status >= http.StatusOK &&
		response.Status < http.StatusMultipleChoices
}
