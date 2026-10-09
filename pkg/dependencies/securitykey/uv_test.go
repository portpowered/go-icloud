package securitykey

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"

	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
)

type cryptoVector struct {
	Version        int    `json:"version"`
	Scalar         string `json:"scalar"`
	PeerX          string `json:"peerX"`
	PeerY          string `json:"peerY"`
	LocalX         string `json:"localX"`
	LocalY         string `json:"localY"`
	Secret         string `json:"secret"`
	EncryptedToken string `json:"encryptedToken"`
	Token          string `json:"token"`
	MAC            string `json:"mac"`
}
type cryptoVectors struct {
	Vectors []cryptoVector `json:"vectors"`
}

func TestPythonPINUVCrypto(test *testing.T) {
	test.Parallel()
	raw, err := os.ReadFile("../../../tests/replay/fixtures/securitykey/python-pinuv-crypto-synthetic.json")
	if err != nil {
		test.Fatal(err)
	}
	var fixture cryptoVectors
	if err := json.Unmarshal(raw, &fixture); err != nil {
		test.Fatal(err)
	}
	for _, vector := range fixture.Vectors {
		test.Run(string(rune('0'+vector.Version)), func(test *testing.T) {
			test.Parallel()
			protocol := wire.PINProtocol(vector.Version)
			peer := &wire.COSEKey{KeyType: wire.COSEKeyKeyTypeN2, Curve: wire.N1, Algorithm: wire.Minus25, X: decodeHex(test, vector.PeerX), Y: decodeHex(test, vector.PeerY)}
			local, secret, err := encapsulate(peer, protocol, bytes.NewReader(decodeHex(test, vector.Scalar)))
			if err != nil {
				test.Fatal(err)
			}
			if !bytes.Equal(local.X, decodeHex(test, vector.LocalX)) || !bytes.Equal(local.Y, decodeHex(test, vector.LocalY)) || !bytes.Equal(secret, decodeHex(test, vector.Secret)) {
				test.Fatal("ECDH/KDF differs from Python")
			}
			token, err := decryptToken(protocol, secret, decodeHex(test, vector.EncryptedToken))
			if err != nil {
				test.Fatal(err)
			}
			if !bytes.Equal(token, decodeHex(test, vector.Token)) {
				test.Fatal("AES-CBC token differs from Python")
			}
		})
	}
}

func TestPINUVRejectInvalidCiphertext(test *testing.T) {
	test.Parallel()
	for _, protocol := range []wire.PINProtocol{wire.PINProtocolV1, wire.PINProtocolV2} {
		for _, size := range []int{0, 1, 15, 17, 48, 80} {
			secret := make([]byte, 32)
			if protocol == wire.PINProtocolV2 {
				secret = make([]byte, 64)
			}
			_, err := decryptToken(protocol, secret, make([]byte, size))
			if protocol == wire.PINProtocolV2 && size == 48 {
				continue
			}
			if !errors.Is(err, ErrProtocol) {
				test.Fatalf("invalid ciphertext accepted: protocol %d, length %d", protocol, size)
			}
		}
	}
}

func TestSourcePINInteractionUnavailable(test *testing.T) {
	test.Parallel()
	options := map[string]bool{string(wire.AlwaysUv): true, string(wire.ClientPin): true}
	channel := &channel{}
	_, err := channel.authParameters(test.Context(), &wire.InfoResponse{Options: &options}, wire.PINProtocolV2, "apple.com", false, true)
	if !errors.Is(err, ErrPINRequired) {
		test.Fatalf("source PIN interaction failure missing: %v", err)
	}
}
