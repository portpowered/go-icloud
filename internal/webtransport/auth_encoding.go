package webtransport

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

// EncodeAuthenticationRequest formats schema-owned authentication models like the pinned source.
func EncodeAuthenticationRequest(input any) ([]byte, error) {
	switch input.(type) {
	case auth.AuthSRPInitRequest, auth.AuthSRPCompleteRequest, auth.AuthTrustedCodeRequest,
		auth.AuthTokenLoginRequest,
		auth.AuthSMSRequest, auth.AuthSMSVerificationRequest, auth.AuthWebAuthnAssertion,
		auth.AuthTrustedDevice, auth.AuthLogoutRequest, auth.AuthCredentialsLoginRequest,
		auth.AuthPCSRequest, auth.AuthBridgeStartRequest, auth.AuthBridgeStepRequest,
		auth.AuthBridgeCodeRequest, auth.AuthGetTermsRequest, auth.AuthAcceptTermsRequest:
		return referenceJSON(input)
	default:
		return nil, fmt.Errorf("encode authentication request: %w", errUnknownAuthRoute)
	}
}

// EncodeBridgeOpaqueData preserves provider-defined metadata and encodes object values inside JSON strings.
func EncodeBridgeOpaqueData(input any) (json.RawMessage, error) {
	if input == nil {
		return nil, nil
	}

	encoded, err := referenceJSON(input)
	if err != nil {
		return nil, err
	}

	var compact bytes.Buffer

	err = json.Compact(&compact, encoded)
	if err != nil {
		return nil, fmt.Errorf("compact bridge metadata: %w", err)
	}

	if bytes.Equal(compact.Bytes(), []byte("null")) {
		return nil, nil
	}

	if len(compact.Bytes()) > 0 && compact.Bytes()[0] == '{' {
		encoded, err = referenceJSON(compact.String())
		if err != nil {
			return nil, err
		}

		return encoded, nil
	}

	return compact.Bytes(), nil
}
