package srp

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"

	models "github.com/portpowered/go-icloud/pkg/dependencymodels/srp"
)

// Challenge derives both SRP proofs after validating the server public value and stretching parameters.
func (client *Client) Challenge(salt, server []byte, iterations int, protocol models.Protocol) (Proof, error) {
	if len(salt) == 0 || len(server) == 0 || len(server) > int(models.RFC5054EphemeralBytes) || iterations < 1 {
		return Proof{}, fmt.Errorf("SRP challenge parameters: %w", ErrChallenge)
	}

	modulus := prime()

	remote := new(big.Int).SetBytes(server)

	if remote.Sign() == 0 || remote.Cmp(modulus) >= 0 {
		return Proof{}, fmt.Errorf("SRP server public value: %w", ErrChallenge)
	}

	password, err := client.stretch(salt, iterations, protocol)
	if err != nil {
		return Proof{}, err
	}

	scrambling := new(big.Int).SetBytes(digest(padded(client.public), padded(remote)))
	if scrambling.Sign() == 0 {
		return Proof{}, fmt.Errorf("SRP scrambling value: %w", ErrChallenge)
	}

	private := new(big.Int).SetBytes(digest(salt, digest([]byte(models.IdentitySeparator), password)))
	key := client.sessionKey(remote, scrambling, private, modulus)

	return client.proof(salt, remote, key), nil
}

func (client *Client) stretch(salt []byte, iterations int, protocol models.Protocol) ([]byte, error) {
	password := client.password[:]

	switch protocol {
	case models.S2k:
	case models.S2kFo:
		password = []byte(hex.EncodeToString(password))
	default:
		return nil, fmt.Errorf("SRP password protocol: %w", ErrChallenge)
	}

	stretched, err := pbkdf2.Key(sha256.New, string(password), salt, iterations, sha256.Size)
	if err != nil {
		return nil, fmt.Errorf("SRP password stretching: %w", err)
	}

	return stretched, nil
}

func (client *Client) sessionKey(remote, scrambling, private, modulus *big.Int) []byte {
	multiplier := new(big.Int).SetBytes(digest(padded(modulus), padded(generator())))
	verifier := new(big.Int).Exp(generator(), private, modulus)
	base := new(big.Int).Sub(remote, new(big.Int).Mul(multiplier, verifier))
	exponent := new(big.Int).Add(client.secret, new(big.Int).Mul(scrambling, private))
	shared := new(big.Int).Exp(base, exponent, modulus)

	return digest(shared.Bytes())
}

func (client *Client) proof(salt []byte, remote *big.Int, key []byte) Proof {
	groupHash := digest(prime().Bytes())

	generatorHash := digest(padded(generator()))

	for index := range groupHash {
		groupHash[index] ^= generatorHash[index]
	}

	first := digest(groupHash, digest([]byte(client.username)), salt, client.public.Bytes(), remote.Bytes(), key)

	return Proof{M1: first, M2: digest(client.public.Bytes(), first, key)}
}
