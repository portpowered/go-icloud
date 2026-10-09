package icloud

import "context"

// UseExistingTrustedDeviceCode selects a code already displayed on a trusted device.
// Close any explicit NativeBridgeSession before selecting this detached verification route.
func (sdk *SDK) UseExistingTrustedDeviceCode(ctx context.Context,
	request NativeAuthRequest,
) (*NativeAuthResult, error) {
	operation, err := newNativeAuthOperation(ctx, "UseExistingTrustedDeviceCode", request.Auth, request.State)
	if err != nil {
		return nil, err
	}
	operation.state.DeliveryNotice = nil
	operation.state.CodeRequested = false
	operation.state.DeliveryMethod = TwoFactorDeliveryTrustedDevice

	return nativeAuthResult(operation)
}
