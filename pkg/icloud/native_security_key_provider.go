package icloud

import (
	"context"
	"fmt"
	"io"

	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/pkg/dependencies/securitykey"
)

type nativeSecurityKeyProvider struct{ provider *securitykey.Provider }

func newNativeSecurityKeyProvider(entropy io.Reader) (*nativeSecurityKeyProvider, error) {
	provider, err := securitykey.New(securitykey.HIDBackend{}, entropy)
	if err != nil {
		return nil, fmt.Errorf("configure security key provider: %w", err)
	}

	return &nativeSecurityKeyProvider{provider: provider}, nil
}

func (provider *nativeSecurityKeyProvider) Devices(ctx context.Context) ([]SecurityKeyDevice, error) {
	devices, err := provider.provider.Devices(ctx)
	if err != nil {
		return nil, fmt.Errorf("discover security key devices: %w", err)
	}

	result := make([]SecurityKeyDevice, 0, len(devices))

	for _, device := range devices {
		result = append(result, SecurityKeyDevice{ID: device.ID, Name: device.Name})
	}

	return result, nil
}

func (provider *nativeSecurityKeyProvider) Assert(
	ctx context.Context, request SecurityKeyCeremony,
) (SecurityKeyAssertion, error) {
	if request.Origin != HttpsappleCom || request.UserVerification != Discouraged {
		return SecurityKeyAssertion{}, errNativeAuthInput
	}

	assertion, err := provider.provider.Assert(ctx, securitykey.Request{
		DeviceID:       request.DeviceID,
		RelyingPartyID: request.Challenge.RelyingPartyID,
		Challenge:      request.Challenge.Challenge,
		Origin:         string(request.Origin),
		CredentialIDs:  request.Challenge.CredentialIDs,
	})
	if err != nil {
		return SecurityKeyAssertion{}, fmt.Errorf("create security key assertion: %w", err)
	}

	result := SecurityKeyAssertion{
		AuthenticatorData: assertion.AuthenticatorData,
		ClientData:        assertion.ClientData,
		CredentialID:      assertion.CredentialID,
		Signature:         assertion.Signature,
		UserHandle:        nullable.NewNullNullable[[]byte](),
	}
	if len(assertion.UserHandle) > 0 {
		result.UserHandle = nullable.NewNullableWithValue(assertion.UserHandle)
	}

	return result, nil
}
