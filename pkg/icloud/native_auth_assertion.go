package icloud

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

const minimumAuthenticatorDataBytes = 37

func nativeValidateAssertion(challenge *SecurityKeyChallenge, assertion SecurityKeyAssertion) error {
	if challenge == nil || challenge.Challenge == "" || challenge.RelyingPartyID == "" || len(assertion.Signature) == 0 ||
		len(assertion.AuthenticatorData) < minimumAuthenticatorDataBytes {
		return errNativeAuthInput
	}

	var data auth.AuthWebAuthnClientData

	if err := json.Unmarshal(assertion.ClientData, &data); err != nil {
		return fmt.Errorf("decode security-key client data: %w", err)
	}

	if string(data.Type) != protocol.AuthWebAuthnClientDataTypeValue || string(data.Origin) != protocol.AuthWebAuthnOriginValue ||
		data.CrossOrigin != nil && *data.CrossOrigin {
		return errNativeAuthInput
	}

	expected, err := nativeDecodeBase64URL(challenge.Challenge)
	if err != nil {
		return err
	}

	actual, err := nativeDecodeBase64URL(data.Challenge)
	if err != nil || !bytes.Equal(expected, actual) {
		return errNativeAuthInput
	}

	hash := sha256.Sum256([]byte(challenge.RelyingPartyID))
	if !bytes.Equal(hash[:], assertion.AuthenticatorData[:sha256.Size]) {
		return errNativeAuthInput
	}

	for _, identifier := range challenge.CredentialIDs {
		allowed, decodeErr := nativeDecodeBase64URL(identifier)
		if decodeErr != nil {
			return decodeErr
		}

		if bytes.Equal(allowed, assertion.CredentialID) && len(allowed) > 0 {
			return nil
		}
	}

	return errNativeAuthInput
}

func nativeDecodeBase64URL(value string) ([]byte, error) {
	if strings.ContainsAny(value, "\r\n") {
		return nil, errNativeAuthInput
	}

	decoded, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil {
		decoded, err = base64.URLEncoding.Strict().DecodeString(value)
	}

	if err != nil {
		return nil, fmt.Errorf("decode security-key challenge: %w", err)
	}

	return decoded, nil
}
