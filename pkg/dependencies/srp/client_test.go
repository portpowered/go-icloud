package srp_test

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencies/srp"
	models "github.com/portpowered/go-icloud/pkg/dependencymodels/srp"
)

// Synthetic offline vectors from pinned pyicloud SrpPassword and srp._pysrp;
// RFC5054 enabled, no_username_in_x enabled, SHA256, NG2048.
//
//go:embed testdata/python-vectors.json
var pythonVectors []byte

type vector struct {
	Username   string          `json:"username"`
	Password   string          `json:"password"`
	Entropy    string          `json:"entropy"`
	Salt       string          `json:"salt"`
	Iterations int             `json:"iterations"`
	Protocol   models.Protocol `json:"protocol"`
	Public     string          `json:"public"`
	Server     string          `json:"server"`
	M1         string          `json:"m1"`
	M2         string          `json:"m2"`
}

func decode(t *testing.T, value string) []byte {
	t.Helper()

	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}

	return decoded
}

func TestPythonProofVectors(t *testing.T) {
	t.Parallel()

	var vectors []vector

	err := json.Unmarshal(pythonVectors, &vectors)
	if err != nil {
		t.Fatal(err)
	}

	for _, fixture := range vectors {
		t.Run(string(fixture.Protocol), func(t *testing.T) {
			t.Parallel()

			client, err := srp.New(fixture.Username, fixture.Password, bytes.NewReader(decode(t, fixture.Entropy)))
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(client.Public(), decode(t, fixture.Public)) {
				t.Fatal("public ephemeral mismatch")
			}

			proof, err := client.Challenge(
				decode(t, fixture.Salt), decode(t, fixture.Server), fixture.Iterations, fixture.Protocol,
			)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(proof.M1, decode(t, fixture.M1)) || !bytes.Equal(proof.M2, decode(t, fixture.M2)) {
				t.Fatal("Python proof mismatch")
			}

			mutated := client.Public()
			mutated[0] ^= 1

			if !bytes.Equal(client.Public(), decode(t, fixture.Public)) {
				t.Fatal("public accessor aliases internal state")
			}
		})
	}
}

func TestInvalidChallenge(t *testing.T) {
	t.Parallel()

	entropy := bytes.NewReader(make([]byte, int(models.RFC5054EphemeralBytes)))

	client, err := srp.New("test@example.invalid", "synthetic", entropy)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name         string
		salt, server []byte
		iterations   int
		protocol     models.Protocol
	}{
		{"empty salt", nil, []byte{1}, 1, models.S2k},
		{"empty server", []byte{1}, nil, 1, models.S2k},
		{"zero server", []byte{1}, []byte{0}, 1, models.S2k},
		{"group prime", []byte{1}, decode(t, string(models.RFC5054Prime)), 1, models.S2k},
		{"oversized server", []byte{1}, make([]byte, int(models.RFC5054EphemeralBytes)+1), 1, models.S2k},
		{"zero iterations", []byte{1}, []byte{1}, 0, models.S2k},
		{"negative iterations", []byte{1}, []byte{1}, -1, models.S2k},
		{"unknown protocol", []byte{1}, []byte{1}, 1, models.Protocol("unknown")},
	}
	for _, testcase := range cases {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			_, failure := client.Challenge(testcase.salt, testcase.server, testcase.iterations, testcase.protocol)
			if !errors.Is(failure, srp.ErrChallenge) {
				t.Fatalf("expected invalid challenge, got %v", failure)
			}
		})
	}
}

func TestEntropyFailure(t *testing.T) {
	t.Parallel()

	_, err := srp.New("test", "synthetic", nil)
	if !errors.Is(err, srp.ErrChallenge) {
		t.Fatal("nil entropy accepted")
	}

	_, err = srp.New("test", "synthetic", bytes.NewReader(nil))
	if !errors.Is(err, io.EOF) {
		t.Fatal("entropy failure lost")
	}
}
