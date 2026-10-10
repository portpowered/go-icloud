package icloud

import (
	"context"
	"errors"
	"slices"

	"github.com/portpowered/go-icloud/pkg/dependencies/securitykey"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

// SecurityKeyAuthenticator owns device discovery and a cancellable hardware assertion ceremony.
// Implementations must bind the selected device, origin and challenge exactly as requested.
type SecurityKeyAuthenticator interface {
	Devices(ctx context.Context) ([]SecurityKeyDevice, error)
	Assert(ctx context.Context, request SecurityKeyCeremony) (SecurityKeyAssertion, error)
}

// WithSecurityKeyAuthenticator supplies caller-owned hardware access safe for concurrent requests.
func WithSecurityKeyAuthenticator(provider SecurityKeyAuthenticator) Option {
	return func(config *configuration) error {
		if provider == nil || config.securityKey != nil {
			return errNativeAuthInput
		}

		config.securityKey = provider

		return nil
	}
}

// ListSecurityKeyDevices enumerates hardware without reading or retaining account credentials.
func (sdk *SDK) ListSecurityKeyDevices(ctx context.Context,
	_ ListSecurityKeyDevicesRequest,
) (*ListSecurityKeyDevicesResult, error) {
	const operation = "ListSecurityKeyDevices"

	contextErr := ctx.Err()
	if contextErr != nil {
		return nil, driveContextFailure(operation, contextErr)
	}

	if sdk.securityKey == nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, errNativeAuthInput)
	}

	devices, err := sdk.securityKey.Devices(ctx)
	if err != nil {
		return nil, nativeSecurityKeyFailure(operation, err)
	}

	return &ListSecurityKeyDevicesResult{Devices: append([]SecurityKeyDevice{}, devices...)}, nil
}

// ConfirmSecurityKey selects a device and submits its assertion against the current MFA challenge.
func (sdk *SDK) ConfirmSecurityKey(ctx context.Context, request ConfirmSecurityKeyRequest) (*NativeAuthResult, error) {
	const operation = "ConfirmSecurityKey"

	progress, validationErr := newNativeAuthOperation(ctx, operation, request.Auth, request.State)
	if validationErr != nil {
		return nil, validationErr
	}

	challenge := progress.state.Challenge.SecurityKeyChallenge
	if challenge == nil || challenge.Challenge == "" || challenge.RelyingPartyID == "" ||
		len(challenge.CredentialIDs) == 0 {
		return nil, newClientError(operation, Configuration, 0, nil, nil, errNativeAuthInput)
	}

	devices, err := sdk.ListSecurityKeyDevices(ctx, ListSecurityKeyDevicesRequest{})
	if err != nil {
		return nil, err
	}

	device, err := nativeSelectSecurityKey(devices.Devices, request.DeviceID)
	if err != nil {
		return nil, newClientError(operation, NoDevices, 0, nil, nil, err)
	}

	challenge.CredentialIDs = slices.Clone(challenge.CredentialIDs)

	ceremony := SecurityKeyCeremony{DeviceID: device.ID, Challenge: *challenge, Origin: HttpsappleCom,
		UserVerification: Discouraged}

	assertion, err := sdk.securityKey.Assert(ctx, ceremony)
	if err != nil {
		return nil, nativeSecurityKeyFailure(operation, err)
	}

	return sdk.VerifySecurityKey(ctx, VerifySecurityKeyRequest{Auth: request.Auth, State: request.State,
		Assertion: assertion})
}

func nativeSelectSecurityKey(devices []SecurityKeyDevice, selected string) (SecurityKeyDevice, error) {
	for _, device := range devices {
		if selected == "" || device.ID == selected {
			return device, nil
		}
	}

	return SecurityKeyDevice{}, errNativeAuthInput
}

// VerifySecurityKey submits a validated caller-owned assertion and refreshes trusted service discovery.
func (sdk *SDK) VerifySecurityKey(ctx context.Context, request VerifySecurityKeyRequest) (*NativeAuthResult, error) {
	operation, err := newNativeAuthOperation(ctx, "VerifySecurityKey", request.Auth, request.State)
	if err != nil {
		return nil, err
	}

	err = nativeValidateAssertion(operation.state.Challenge.SecurityKeyChallenge, request.Assertion)
	if err != nil {
		return nil, newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	challenge := operation.state.Challenge.SecurityKeyChallenge
	input := auth.AuthWebAuthnAssertion{Challenge: challenge.Challenge, ClientData: request.Assertion.ClientData,
		SignatureData: request.Assertion.Signature, AuthenticatorData: request.Assertion.AuthenticatorData,
		UserHandle: request.Assertion.UserHandle, CredentialID: request.Assertion.CredentialID,
		RpId: challenge.RelyingPartyID}

	wire, err := nativeEncodedRequest(webtransport.VerifyAuthSecurityKeyCall{
		Origin: nativeIDMSOrigin(operation.state), Params: nil, Body: input,
	})
	if err != nil {
		return nil, newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, wire,
		nativeAuthHeaders(operation.state, nativeJSONMedia()))
	if err != nil {
		return nil, err
	}

	err = nativeRequireSuccess(operation, response)
	if err != nil {
		return nil, err
	}

	err = sdk.nativeTrust(ctx, operation)
	if err != nil {
		return nil, err
	}

	return nativeAuthResult(operation)
}

func nativeSecurityKeyFailure(operation string, err error) *ClientError {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return driveContextFailure(operation, err)
	}

	kind := Transport

	var deviceError *securitykey.Error

	switch {
	case errors.Is(err, securitykey.ErrPINRequired):
		kind = AuthenticationRequired
	case errors.Is(err, securitykey.ErrUnsupported):
		kind = Configuration
	case errors.As(err, &deviceError) && deviceError.Status != 0:
		kind = Provider
	case errors.Is(err, securitykey.ErrProtocol):
		kind = InvalidResponse
	}

	return newClientError(operation, kind, 0, nil, nil, err)
}
