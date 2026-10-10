package bridgeprover_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/dependencies/bridgeprover"
)

// These are deterministic synthetic outputs from pinned hsa2_bridge_prover.py,
// with scalar 42, salt "synthetic bridge salt", and nonce bytes 0 through 11.
const (
	salt   = "c3ludGhldGljIGJyaWRnZSBzYWx0"
	shareP = "04fc30efe89c6f7725e0f63829cb33f8e352cb4dffe4af21772f0b3350ac651344" +
		"03d925be2ccda6ed669d4e175e99748d93efc5452e180431d96045b041736d77"
	shareV = "0448495ec0ff0cbe6106180ccff6c7608ea4a84e7a423d83edd9e087dff9d4a7c3" +
		"668ab9426535c8ad435d17059c31392831956224d749b866f00e5ed1d9f0308a"
	confirmP   = "099a63dd3cee72181f249c46771fd89db096763765c2b72b1092f637e2142e70"
	confirmV   = "0156ff97b9b457bb434fbb490ec9e913b9c1e41cfa990d5f8b34dfaedb4cf0d7"
	sharedKey  = "067efe29cbdfe61ce5f56b0198721dca6a9f176ac220a1bea91e58b905fba662"
	ciphertext = "AAABAgMEBQYHCAkKCzr1jVr/zVTK/4z70gGZWpOo8wzFN8I="
)

func initialized(t *testing.T) *bridgeprover.Prover {
	t.Helper()

	entropy := make([]byte, 32)
	entropy[len(entropy)-1] = 42

	prover := bridgeprover.New(bytes.NewReader(entropy))

	err := prover.Init(salt, "654321")
	if err != nil {
		t.Fatal(err)
	}

	return prover
}

func established(t *testing.T) *bridgeprover.Prover {
	t.Helper()
	prover := initialized(t)

	actual, err := prover.Message1()
	if err != nil || actual != shareP {
		t.Fatalf("message1 = %s, %v", actual, err)
	}

	actual, err = prover.ProcessMessage1(shareV)
	if err != nil || actual != confirmP {
		t.Fatalf("message2 = %s, %v", actual, err)
	}

	return prover
}

func TestPinnedPythonVector(t *testing.T) {
	t.Parallel()
	prover := established(t)

	actual, err := prover.ProcessMessage2(confirmV)
	if err != nil || actual != sharedKey {
		t.Fatalf("shared key = %s, %v", actual, err)
	}

	if !prover.Verified() {
		t.Fatal("verification not recorded")
	}

	actual, err = prover.Key()
	if err != nil || actual != sharedKey {
		t.Fatalf("key = %s, %v", actual, err)
	}

	actual, err = prover.DecryptMessage(ciphertext)
	if err != nil || actual != "987654" {
		t.Fatalf("plaintext = %s, %v", actual, err)
	}
}

func TestRejectsMalformedSharesAndConfirmation(t *testing.T) {
	t.Parallel()

	invalid := []string{
		"", "00", "04ff", "zz", "02ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	}
	for _, share := range invalid {
		prover := initialized(t)

		_, err := prover.Message1()
		if err != nil {
			t.Fatal(err)
		}

		_, err = prover.ProcessMessage1(share)
		if !errors.Is(err, bridgeprover.ErrPayload) {
			t.Fatalf("share %q error %v", share, err)
		}
	}

	prover := established(t)

	_, err := prover.ProcessMessage2(confirmP)
	if !errors.Is(err, bridgeprover.ErrPayload) {
		t.Fatalf("confirmation error %v", err)
	}

	if prover.Verified() {
		t.Fatal("invalid confirmation marked verified")
	}

	_, callErr1 := prover.Key()
	if !errors.Is(callErr1, bridgeprover.ErrState) {
		t.Fatalf("unverified key callErr1or %v", callErr1)
	}
}

func TestRejectsMalformedCiphertext(t *testing.T) {
	t.Parallel()

	prover := established(t)

	_, callErr2 := prover.ProcessMessage2(confirmV)
	if callErr2 != nil {
		t.Fatal(callErr2)
	}

	payload, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		t.Fatal(err)
	}

	for _, offset := range []int{0, 1, 13, 29, len(payload) - 1} {
		mutated := bytes.Clone(payload)

		mutated[offset] ^= 1

		_, callErr3 := prover.DecryptMessage(base64.StdEncoding.EncodeToString(mutated))
		if callErr3 == nil {
			t.Fatalf("accepted mutation %d", offset)
		}
	}

	for _, value := range []string{"", "AA==", "!"} {
		_, callErr4 := prover.DecryptMessage(value)
		if callErr4 == nil {
			t.Fatalf("accepted ciphertext %q", value)
		}
	}
}

func TestStateAndEntropyFailures(t *testing.T) {
	t.Parallel()

	prover := bridgeprover.New(bytes.NewReader(nil))

	err := prover.Init(salt, "654321")
	if err == nil {
		t.Fatal("accepted exhausted entropy")
	}

	err = prover.Init("!", "654321")
	if err == nil {
		t.Fatal("accepted malformed salt")
	}

	_, callErr5 := prover.Message1()
	if !errors.Is(callErr5, bridgeprover.ErrState) {
		t.Fatal(callErr5)
	}

	_, callErr6 := prover.ProcessMessage1(shareV)
	if !errors.Is(callErr6, bridgeprover.ErrState) {
		t.Fatal(callErr6)
	}

	_, callErr7 := prover.Message2()
	if !errors.Is(callErr7, bridgeprover.ErrState) {
		t.Fatal(callErr7)
	}

	_, callErr8 := prover.ProcessMessage2(confirmV)
	if !errors.Is(callErr8, bridgeprover.ErrState) {
		t.Fatal(callErr8)
	}

	_, callErr9 := prover.DecryptMessage(ciphertext)
	if !errors.Is(callErr9, bridgeprover.ErrState) {
		t.Fatal(callErr9)
	}
}

func TestSourcePermissiveBase64AndInvalidPlaintext(t *testing.T) {
	t.Parallel()
	prover := established(t)

	_, err := prover.ProcessMessage2(confirmV)
	if err != nil {
		t.Fatal(err)
	}

	actual, err := prover.DecryptMessage("! " + ciphertext[:8] + " \n" + ciphertext[8:] + "junk")
	if err != nil || actual != "987654" {
		t.Fatalf("Source ASCII punctuation = %q, %v", actual, err)
	}
	// Synthetic pinned-Python AES-GCM output with plaintext byte FF.
	_, err = prover.DecryptMessage("AAABAgMEBQYHCAkKCwR3kup6JjGZGsCulaisgSlu")
	if !errors.Is(err, bridgeprover.ErrPayload) {
		t.Fatalf("invalid UTF-8 plaintext = %v", err)
	}

	_, err = prover.DecryptMessage(ciphertext + "\u00e9")
	if !errors.Is(err, bridgeprover.ErrPayload) {
		t.Fatalf("non-ASCII ciphertext = %v", err)
	}
}

type proverZeroEntropy struct{}

func (proverZeroEntropy) Read(payload []byte) (int, error) {
	clear(payload)

	return len(payload), nil
}

func TestInitContextCancelsZeroScalarRetries(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	prover := bridgeprover.New(proverZeroEntropy{})

	err := prover.InitContext(ctx, salt, "654321")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("zero scalar cancellation = %v", err)
	}
}

func TestInitContextRejectsAlreadyCanceledWork(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	prover := bridgeprover.New(bytes.NewReader(nil))

	err := prover.InitContext(ctx, salt, "654321")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("initial cancellation = %v", err)
	}
}
