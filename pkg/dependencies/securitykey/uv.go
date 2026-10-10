package securitykey

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"io"
	"slices"

	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
)

type authParameters struct {
	protocol   wire.PINProtocol
	token      []byte
	internalUV bool
}

// The absence of a negotiated PIN protocol is local policy, not a wire value.
type pinProtocolNegotiation struct {
	protocol  wire.PINProtocol
	supported bool
}

func infoOption(info *wire.InfoResponse, option wire.InfoOption) bool {
	return info.Options != nil && (*info.Options)[string(option)]
}

func negotiatedProtocol(info *wire.InfoResponse) pinProtocolNegotiation {
	if info.PinUvAuthProtocols == nil {
		return pinProtocolNegotiation{protocol: wire.PINProtocolV1, supported: false}
	}

	for _, candidate := range []wire.PINProtocol{wire.PINProtocolV2, wire.PINProtocolV1} {
		if slices.Contains(*info.PinUvAuthProtocols, int(candidate)) {
			return pinProtocolNegotiation{protocol: candidate, supported: true}
		}
	}

	return pinProtocolNegotiation{protocol: wire.PINProtocolV1, supported: false}
}

func (channel *channel) authParameters(
	ctx context.Context, info *wire.InfoResponse, negotiation pinProtocolNegotiation, relyingPartyID string,
	required, allowUV bool,
) (authParameters, error) {
	result := authParameters{
		protocol:   negotiation.protocol,
		token:      nil,
		internalUV: false,
	}
	if !required && !infoOption(info, wire.AlwaysUv) {
		return result, nil
	}

	if allowUV && infoOption(info, wire.Uv) {
		if !infoOption(info, wire.PinUvAuthToken) {
			result.internalUV = true

			return result, nil
		}

		if !negotiation.supported || !negotiation.protocol.Valid() {
			return result, keyFailure("PIN protocol", ErrUnsupported)
		}

		token, err := channel.uvToken(ctx, negotiation.protocol, relyingPartyID)
		result.token = token

		return result, err
	}

	if infoOption(info, wire.ClientPin) {
		return result, keyFailure("PIN interaction", ErrPINRequired)
	}

	return result, keyFailure("user verification", ErrUnsupported)
}

func (parameters authParameters) apply(request *wire.AssertionRequest) {
	if len(parameters.token) > 0 {
		mac := hmac.New(sha256.New, parameters.token)
		_, _ = mac.Write(request.ClientDataHash)

		authentication := mac.Sum(nil)
		if parameters.protocol == wire.PINProtocolV1 {
			authentication = authentication[:aes.BlockSize]
		}

		protocol := int(parameters.protocol)
		request.PinUvAuthParam = &authentication
		request.PinUvAuthProtocol = &protocol
	}
}

func (channel *channel) uvToken(ctx context.Context, protocol wire.PINProtocol, relyingPartyID string) ([]byte, error) {
	var agreement wire.PINResponse

	err := channel.call(ctx, wire.ClientPIN, wire.PINRequest{
		Protocol:        protocol,
		Command:         wire.GetKeyAgreement,
		KeyAgreement:    nil,
		Permissions:     nil,
		PermissionsRpId: nil,
	}, &agreement)
	if err != nil {
		return nil, err
	}

	local, secret, err := encapsulate(agreement.KeyAgreement, protocol, channel.entropy)
	if err != nil {
		return nil, err
	}

	permission := wire.PINRequestPermissionsN2

	var response wire.PINResponse

	request := wire.PINRequest{
		Protocol:        protocol,
		Command:         wire.GetUVToken,
		KeyAgreement:    local,
		Permissions:     &permission,
		PermissionsRpId: &relyingPartyID,
	}

	err = channel.call(ctx, wire.ClientPIN, request, &response)
	if err != nil {
		return nil, err
	}

	if response.Token == nil {
		return nil, keyFailure("UV token", ErrProtocol)
	}

	return decryptToken(protocol, secret, *response.Token)
}

func encapsulate(peer *wire.COSEKey, protocol wire.PINProtocol, entropy io.Reader) (*wire.COSEKey, []byte, error) {
	if !validAgreement(peer) {
		return nil, nil, keyFailure("key agreement", ErrProtocol)
	}

	publicBytes := append(append([]byte{4}, peer.X...), peer.Y...)

	public, err := ecdh.P256().NewPublicKey(publicBytes)
	if err != nil {
		return nil, nil, keyFailure("key agreement", err)
	}
	// Read an explicit scalar so deterministic injected entropy has no optional random reads.
	scalar := make([]byte, sha256.Size)

	_, err = io.ReadFull(entropy, scalar)
	if err != nil {
		return nil, nil, keyFailure("key entropy", err)
	}

	private, err := ecdh.P256().NewPrivateKey(scalar)
	if err != nil {
		return nil, nil, keyFailure("key scalar", err)
	}

	shared, err := private.ECDH(public)
	if err != nil {
		return nil, nil, keyFailure("ECDH", err)
	}

	secret, err := deriveSecret(protocol, shared)
	if err != nil {
		return nil, nil, err
	}

	encoded := private.PublicKey().Bytes()
	local := &wire.COSEKey{
		KeyType:   wire.COSEKeyKeyTypeN2,
		Algorithm: wire.Minus25,
		Curve:     wire.N1,
		X:         encoded[1 : sha256.Size+1],
		Y:         encoded[sha256.Size+1:],
	}

	return local, secret, nil
}

func deriveSecret(protocol wire.PINProtocol, shared []byte) ([]byte, error) {
	if protocol == wire.PINProtocolV1 {
		hash := sha256.Sum256(shared)

		return hash[:], nil
	}

	if protocol != wire.PINProtocolV2 {
		return nil, keyFailure("PIN protocol", ErrProtocol)
	}

	salt := make([]byte, sha256.Size)

	hmacKey, err := hkdf.Key(sha256.New, shared, salt, string(wire.HMACKeyLabel), sha256.Size)
	if err != nil {
		return nil, keyFailure("HKDF", err)
	}

	aesKey, err := hkdf.Key(sha256.New, shared, salt, string(wire.AESKeyLabel), sha256.Size)
	if err != nil {
		return nil, keyFailure("HKDF", err)
	}

	return append(hmacKey, aesKey...), nil
}

func decryptToken(protocol wire.PINProtocol, secret, ciphertext []byte) ([]byte, error) {
	key := secret

	initializationVector := make([]byte, aes.BlockSize)

	if protocol == wire.PINProtocolV2 {
		if len(secret) != sha256.Size*2 || len(ciphertext) <= aes.BlockSize {
			return nil, keyFailure("UV ciphertext", ErrProtocol)
		}

		key = secret[sha256.Size:]
		initializationVector = ciphertext[:aes.BlockSize]
		ciphertext = ciphertext[aes.BlockSize:]
	}

	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, keyFailure("UV ciphertext", ErrProtocol)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, keyFailure("UV cipher", err)
	}

	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, initializationVector).CryptBlocks(plaintext, ciphertext)

	if !validTokenLength(protocol, len(plaintext)) {
		return nil, keyFailure("UV token length", ErrProtocol)
	}

	return plaintext, nil
}

func validAgreement(peer *wire.COSEKey) bool {
	return peer != nil && peer.KeyType.Valid() && peer.Curve.Valid() &&
		len(peer.X) == sha256.Size && len(peer.Y) == sha256.Size
}

func validTokenLength(protocol wire.PINProtocol, length int) bool {
	if protocol == wire.PINProtocolV1 {
		return length == aes.BlockSize || length == sha256.Size
	}

	return protocol == wire.PINProtocolV2 && length == sha256.Size
}
