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

	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
)

type authParameters struct {
	protocol   wire.PINProtocol
	token      []byte
	internalUV bool
}

func infoOption(info *wire.InfoResponse, option wire.InfoOption) bool {
	return info.Options != nil && (*info.Options)[string(option)]
}

func negotiatedProtocol(info *wire.InfoResponse) wire.PINProtocol {
	if info.PinUvAuthProtocols == nil {
		return 0
	}
	for _, candidate := range []wire.PINProtocol{wire.PINProtocolV2, wire.PINProtocolV1} {
		for _, supported := range *info.PinUvAuthProtocols {
			if int(candidate) == supported {
				return candidate
			}
		}
	}
	return 0
}

func (channel *channel) authParameters(ctx context.Context, info *wire.InfoResponse, protocol wire.PINProtocol, rp string, required, allowUV bool) (authParameters, error) {
	result := authParameters{protocol: protocol}
	if !required && !infoOption(info, wire.AlwaysUv) {
		return result, nil
	}
	if allowUV && infoOption(info, wire.Uv) {
		if !infoOption(info, wire.PinUvAuthToken) {
			result.internalUV = true
			return result, nil
		}
		if !protocol.Valid() {
			return result, &Error{Stage: "PIN protocol", Cause: ErrUnsupported}
		}
		token, err := channel.uvToken(ctx, protocol, rp)
		result.token = token
		return result, err
	}
	if infoOption(info, wire.ClientPin) {
		return result, &Error{Stage: "PIN interaction", Cause: ErrPINRequired}
	}
	return result, &Error{Stage: "user verification", Cause: ErrUnsupported}
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

func (channel *channel) uvToken(ctx context.Context, protocol wire.PINProtocol, rp string) ([]byte, error) {
	var agreement wire.PINResponse
	if err := channel.call(ctx, wire.ClientPIN, wire.PINRequest{Protocol: protocol, Command: wire.GetKeyAgreement}, &agreement); err != nil {
		return nil, err
	}
	local, secret, err := encapsulate(agreement.KeyAgreement, protocol, channel.entropy)
	if err != nil {
		return nil, err
	}
	permission := wire.PINRequestPermissionsN2
	var response wire.PINResponse
	request := wire.PINRequest{Protocol: protocol, Command: wire.GetUVToken, KeyAgreement: local, Permissions: &permission, PermissionsRpId: &rp}
	if err := channel.call(ctx, wire.ClientPIN, request, &response); err != nil {
		return nil, err
	}
	if response.Token == nil {
		return nil, &Error{Stage: "UV token", Cause: ErrProtocol}
	}
	return decryptToken(protocol, secret, *response.Token)
}

func encapsulate(peer *wire.COSEKey, protocol wire.PINProtocol, entropy io.Reader) (*wire.COSEKey, []byte, error) {
	if peer == nil || !peer.KeyType.Valid() || !peer.Curve.Valid() || len(peer.X) != sha256.Size || len(peer.Y) != sha256.Size {
		return nil, nil, &Error{Stage: "key agreement", Cause: ErrProtocol}
	}
	publicBytes := append(append([]byte{4}, peer.X...), peer.Y...)
	public, err := ecdh.P256().NewPublicKey(publicBytes)
	if err != nil {
		return nil, nil, &Error{Stage: "key agreement", Cause: err}
	}
	// Read an explicit scalar so deterministic injected entropy has no optional random reads.
	scalar := make([]byte, sha256.Size)
	if _, err := io.ReadFull(entropy, scalar); err != nil {
		return nil, nil, &Error{Stage: "key entropy", Cause: err}
	}
	private, err := ecdh.P256().NewPrivateKey(scalar)
	if err != nil {
		return nil, nil, &Error{Stage: "key scalar", Cause: err}
	}
	shared, err := private.ECDH(public)
	if err != nil {
		return nil, nil, &Error{Stage: "ECDH", Cause: err}
	}
	secret, err := deriveSecret(protocol, shared)
	if err != nil {
		return nil, nil, err
	}
	encoded := private.PublicKey().Bytes()
	local := &wire.COSEKey{KeyType: wire.COSEKeyKeyTypeN2, Algorithm: wire.Minus25, Curve: wire.N1, X: encoded[1 : sha256.Size+1], Y: encoded[sha256.Size+1:]}
	return local, secret, nil
}

func deriveSecret(protocol wire.PINProtocol, shared []byte) ([]byte, error) {
	if protocol == wire.PINProtocolV1 {
		hash := sha256.Sum256(shared)
		return hash[:], nil
	}
	if protocol != wire.PINProtocolV2 {
		return nil, &Error{Stage: "PIN protocol", Cause: ErrProtocol}
	}
	salt := make([]byte, sha256.Size)
	hmacKey, err := hkdf.Key(sha256.New, shared, salt, string(wire.HMACKeyLabel), sha256.Size)
	if err != nil {
		return nil, &Error{Stage: "HKDF", Cause: err}
	}
	aesKey, err := hkdf.Key(sha256.New, shared, salt, string(wire.AESKeyLabel), sha256.Size)
	if err != nil {
		return nil, &Error{Stage: "HKDF", Cause: err}
	}
	return append(hmacKey, aesKey...), nil
}

func decryptToken(protocol wire.PINProtocol, secret, ciphertext []byte) ([]byte, error) {
	key := secret
	iv := make([]byte, aes.BlockSize)
	if protocol == wire.PINProtocolV2 {
		if len(secret) != sha256.Size*2 || len(ciphertext) <= aes.BlockSize {
			return nil, &Error{Stage: "UV ciphertext", Cause: ErrProtocol}
		}
		key = secret[sha256.Size:]
		iv = ciphertext[:aes.BlockSize]
		ciphertext = ciphertext[aes.BlockSize:]
	}
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, &Error{Stage: "UV ciphertext", Cause: ErrProtocol}
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, &Error{Stage: "UV cipher", Cause: err}
	}
	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, ciphertext)
	if (protocol == wire.PINProtocolV1 && len(plaintext) != aes.BlockSize && len(plaintext) != sha256.Size) || (protocol == wire.PINProtocolV2 && len(plaintext) != sha256.Size) {
		return nil, &Error{Stage: "UV token length", Cause: ErrProtocol}
	}
	return plaintext, nil
}
