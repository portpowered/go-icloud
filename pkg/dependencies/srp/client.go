// Package srp implements Apple's password SRP-6a proof calculation.
package srp

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math/big"

	models "github.com/portpowered/go-icloud/pkg/dependencymodels/srp"
)

const hexadecimalBase = 16

// ErrChallenge identifies an invalid or unsupported SRP challenge.
var ErrChallenge = errors.New("invalid SRP challenge")

// Client owns one account's ephemeral secret and password digest. It must not be shared concurrently.
type Client struct {
	username string
	password [sha256.Size]byte
	secret   *big.Int
	public   *big.Int
}

// Proof contains the client proof and expected server proof. Neither contains the session key.
type Proof struct {
	M1 []byte
	M2 []byte
}

// New generates a fresh 2048-bit secret using the injected entropy reader.
func New(username, password string, entropy io.Reader) (*Client, error) {
	if entropy == nil {
		return nil, fmt.Errorf("SRP entropy reader: %w", ErrChallenge)
	}

	secret := make([]byte, int(models.RFC5054EphemeralBytes))

	_, err := io.ReadFull(entropy, secret)
	if err != nil {
		return nil, fmt.Errorf("SRP entropy: %w", err)
	}

	secret[0] |= byte(models.EphemeralHighBit)
	value := new(big.Int).SetBytes(secret)

	return &Client{username: username, password: sha256.Sum256([]byte(password)), secret: value,
		public: new(big.Int).Exp(generator(), value, prime())}, nil
}

// Public returns the minimal big-endian public ephemeral value, detached from the client.
func (client *Client) Public() []byte { return client.public.Bytes() }

func prime() *big.Int {
	value, _ := new(big.Int).SetString(string(models.RFC5054Prime), hexadecimalBase)

	return value
}

func generator() *big.Int { return big.NewInt(int64(models.RFC5054Generator)) }

func digest(parts ...[]byte) []byte {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write(part)
	}

	return hash.Sum(nil)
}

func padded(value *big.Int) []byte {
	return value.FillBytes(make([]byte, int(models.RFC5054EphemeralBytes)))
}
