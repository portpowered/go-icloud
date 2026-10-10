//nolint:testpackage // White-box protocol and injected HID lifecycle tests require unexported seams (GO-15).
package securitykey

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strconv"
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

func TestPythonPINUVCrypto(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../../tests/replay/fixtures/securitykey/python-pinuv-crypto-synthetic.json")
	if err != nil {
		t.Fatal(err)
	}

	var fixture cryptoVectors

	err = json.Unmarshal(raw, &fixture)
	if err != nil {
		t.Fatal(err)
	}

	for _, vector := range fixture.Vectors {
		t.Run(strconv.Itoa(vector.Version), func(t *testing.T) {
			t.Parallel()

			protocol := wire.PINProtocol(vector.Version)
			peer := &wire.COSEKey{
				KeyType:   wire.COSEKeyKeyTypeN2,
				Curve:     wire.N1,
				Algorithm: wire.Minus25,
				X:         decodeHex(t, vector.PeerX),
				Y:         decodeHex(t, vector.PeerY),
			}

			local, secret, err := encapsulate(peer, protocol, bytes.NewReader(decodeHex(t, vector.Scalar)))
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(local.X, decodeHex(t, vector.LocalX)) ||
				!bytes.Equal(local.Y, decodeHex(t, vector.LocalY)) ||
				!bytes.Equal(secret, decodeHex(t, vector.Secret)) {
				t.Fatal("ECDH/KDF differs from Python")
			}

			token, err := decryptToken(protocol, secret, decodeHex(t, vector.EncryptedToken))
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(token, decodeHex(t, vector.Token)) {
				t.Fatal("AES-CBC token differs from Python")
			}
		})
	}
}

func TestPINUVRejectInvalidCiphertext(t *testing.T) {
	t.Parallel()

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
				t.Fatalf("invalid ciphertext accepted: protocol %d, length %d", protocol, size)
			}
		}
	}
}

func TestSourcePINInteractionUnavailable(t *testing.T) {
	t.Parallel()

	options := map[string]bool{string(wire.AlwaysUv): true, string(wire.ClientPin): true}
	channel := &channel{
		connection:      nil,
		id:              0,
		ctap2:           false,
		wait:            nil,
		entropy:         nil,
		maxMessageBytes: 0,
	}

	_, err := channel.authParameters(t.Context(), &wire.InfoResponse{
		Options:                  &options,
		Aaguid:                   nil,
		MaxCredentialCountInList: nil,
		MaxCredentialIdLength:    nil,
		MaxMsgSize:               nil,
		PinUvAuthProtocols:       nil,
		Versions:                 nil,
	}, pinProtocolNegotiation{protocol: wire.PINProtocolV2, supported: true}, syntheticRelyingPartyID, false, true)
	if !errors.Is(err, ErrPINRequired) {
		t.Fatalf("source PIN interaction failure missing: %v", err)
	}
}
