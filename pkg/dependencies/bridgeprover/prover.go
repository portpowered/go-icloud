// Package bridgeprover implements the pinned trusted-device SPAKE2 bridge worker.
// One Prover belongs to one verification attempt and must not be used concurrently.
package bridgeprover

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"unicode/utf8"

	model "github.com/portpowered/go-icloud/pkg/dependencymodels/bridgeprover"
	"golang.org/x/crypto/scrypt"
)

// ErrState identifies an out-of-order bridge operation.
var ErrState = errors.New("bridge prover operation out of order")

// ErrPayload identifies malformed or unauthenticated bridge data.
var ErrPayload = errors.New("invalid bridge payload")

// Prover owns ephemeral state for one bridge verification attempt.
type Prover struct {
	entropy                                                        io.Reader
	x, w0, w1                                                      *big.Int
	shareP, shareV                                                 []byte
	confirmationClient, confirmationServer, sharedKey, verifierKey []byte
	verified                                                       bool
}

// New creates a prover with caller-owned entropy. A nil reader uses crypto/rand.
func New(entropy io.Reader) *Prover {
	if entropy == nil {
		entropy = rand.Reader
	}

	prover := new(Prover)
	prover.entropy = entropy

	return prover
}

// Init derives password scalars and clears any previous verification state.
func (p *Prover) Init(saltBase64, code string) error {
	return p.InitContext(context.Background(), saltBase64, code)
}

// InitContext also cancels entropy rejection retries and checks password derivation.
func (p *Prover) InitContext(ctx context.Context, saltBase64, code string) error {
	contextError := ctx.Err()
	if contextError != nil {
		return fmt.Errorf("bridge initialization: %w", contextError)
	}

	salt, err := decodeBase64(saltBase64)
	if err != nil {
		return fmt.Errorf("bridge salt: %w", err)
	}

	key, err := scrypt.Key(
		[]byte(code), salt, int(model.ScryptCost), int(model.ScryptBlockSize),
		int(model.ScryptParallelism), int(model.PasswordKeyLength),
	)
	if err != nil {
		return fmt.Errorf("bridge password derivation: %w", err)
	}

	scalar, err := randomScalar(ctx, p.entropy)
	if err != nil {
		return err
	}

	p.x = scalar
	midpoint := int(model.PasswordScalarBytes)
	p.w0 = new(big.Int).SetBytes(key[:midpoint])
	p.w1 = new(big.Int).SetBytes(key[midpoint:])
	p.shareP, p.shareV = nil, nil
	p.confirmationClient, p.confirmationServer, p.sharedKey, p.verifierKey = nil, nil, nil, nil
	p.verified = false

	return nil
}

func randomScalar(ctx context.Context, entropy io.Reader) (*big.Int, error) {
	for {
		contextError := ctx.Err()
		if contextError != nil {
			return nil, fmt.Errorf("bridge entropy: %w", contextError)
		}

		scalar, err := rand.Int(entropy, elliptic.P256().Params().N)
		if err != nil {
			return nil, fmt.Errorf("bridge entropy: %w", err)
		}

		if scalar.Sign() != 0 {
			return scalar, nil
		}
	}
}

// Message1 returns the prover's uncompressed SEC1 share.
func (p *Prover) Message1() (string, error) {
	if p.x == nil {
		return "", ErrState
	}

	curve := elliptic.P256()

	maskMX, maskMY, err := decodePoint(string(model.SPAKEM))
	if err != nil {
		return "", err
	}

	x, y := curve.ScalarBaseMult(p.x.Bytes())
	maskX, maskY := curve.ScalarMult(maskMX, maskMY, scalarBytes(p.w0))
	x, y = curve.Add(x, y, maskX, maskY)
	p.shareP, err = encodePoint(x, y)

	return hex.EncodeToString(p.shareP), err
}

// ProcessMessage1 consumes the verifier share and returns the client confirmation.
func (p *Prover) ProcessMessage1(message string) (string, error) {
	if len(p.shareP) == 0 {
		return "", ErrState
	}

	transcript, err := p.transcript(message)
	if err != nil {
		return "", err
	}

	digest := sha256.Sum256(transcript)
	confirmations := derive(digest[:], string(model.ConfirmationInfo), int(model.ConfirmationKeyBytes))
	midpoint := int(model.ConfirmationHalfBytes)
	p.confirmationClient, p.confirmationServer = confirmations[:midpoint], confirmations[midpoint:]
	p.sharedKey = derive(digest[:], string(model.SharedInfo), int(model.BridgeKeyLength))

	return p.Message2()
}

// Message2 returns the prover's HMAC confirmation.
func (p *Prover) Message2() (string, error) {
	if p.confirmationClient == nil {
		return "", ErrState
	}

	return hex.EncodeToString(mac(p.confirmationClient, p.shareV)), nil
}

// ProcessMessage2 verifies the server confirmation and returns the shared key.
func (p *Prover) ProcessMessage2(message string) (string, error) {
	if p.confirmationServer == nil {
		return "", ErrState
	}

	expected := hex.EncodeToString(mac(p.confirmationServer, p.shareP))
	if !hmac.Equal([]byte(expected), []byte(message)) {
		return "", ErrPayload
	}

	p.verifierKey = derive(p.sharedKey, string(model.VerifierInfo), int(model.BridgeKeyLength))
	p.verified = true

	return hex.EncodeToString(p.sharedKey), nil
}

// Verified reports whether the verifier confirmation has passed.
func (p *Prover) Verified() bool { return p.verified }

// Key returns the verified shared key, never unconfirmed key material.
func (p *Prover) Key() (string, error) {
	if !p.verified {
		return "", ErrState
	}

	return hex.EncodeToString(p.sharedKey), nil
}

// DecryptMessage authenticates the version, nonce, tag and encrypted validation code.
func (p *Prover) DecryptMessage(message string) (string, error) {
	if !p.verified {
		return "", ErrState
	}

	payload, err := decodeBase64(message)
	if err != nil {
		return "", fmt.Errorf("bridge ciphertext: %w", err)
	}

	prefix := 1 + int(model.GCMNonceLength)

	end := prefix + int(model.GCMTagLength)
	if len(payload) < end || payload[0] != byte(model.GCMVersion) {
		return "", ErrPayload
	}

	aead, err := newGCM(p.verifierKey)
	if err != nil {
		return "", err
	}

	ciphertext := append(append([]byte(nil), payload[end:]...), payload[prefix:end]...)

	plaintext, err := aead.Open(nil, payload[1:prefix], ciphertext, payload[:1])
	if err != nil {
		return "", fmt.Errorf("bridge authentication: %w", errors.Join(ErrPayload, err))
	}

	if !utf8.Valid(plaintext) {
		return "", ErrPayload
	}

	return string(plaintext), nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("bridge AES: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("bridge GCM: %w", err)
	}

	return aead, nil
}

func mac(key, message []byte) []byte {
	hash := hmac.New(sha256.New, key)
	hash.Write(message)

	return hash.Sum(nil)
}

func derive(key []byte, info string, length int) []byte {
	extracted := mac(make([]byte, sha256.Size), key)

	output, previous := []byte{}, []byte{}
	for counter := byte(1); len(output) < length; counter++ {
		message := append(append(append([]byte(nil), previous...), []byte(info)...), counter)
		previous = mac(extracted, message)
		output = append(output, previous...)
	}

	return output[:length]
}
