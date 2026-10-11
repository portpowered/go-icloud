package replay_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const interactiveReplayToken = "synthetic-srp-token"
const interactiveReplayCookie = "synthetic-auth-value"

func TestInteractiveLoginCompletesTrustedDeviceReplay(t *testing.T) {
	t.Parallel()

	path := "../../../../tests/replay/fixtures/synthetic/http/auth-bridge-full-modern-204.json"
	raw := readObject(t, path)
	runner, transport, sdk := nativeBridgeCommandReplay(t, raw, false)
	client := &nativeBridgeCommandClient{Client: sdk, runner: runner, owner: nil}
	state := nativeBridgeCommandState(t, raw)
	account := state.AccountName
	// Prompt for the account as well as password and code, preserving the fixture's
	// stable browser identity without preloading customer credentials.
	state.AccountName = ""
	savedPath := interactiveReplaySession(t, state)

	var output, diagnostic bytes.Buffer

	err := command.RunWithInput(t.Context(), client, []string{"auth", "login", sessionFlag, savedPath},
		io.NopCloser(strings.NewReader(account+"\n"+expectedInventedPassword+"\n123456\n")),
		func(string) string { return "" }, &output, &diagnostic)
	if err != nil || client.owner == nil {
		t.Fatalf("interactive device login failed: %v", err)
	}

	nativeBridgeCommandSavedState(t, raw, path, savedPath, client.owner, false, false)
	nativeBridgeCommandConsumed(t, runner, transport)
	assertNativeCommandPrivacy(t, output.String()+diagnostic.String(), []string{
		account, expectedInventedPassword, "123456", expectedSyntheticSessionValue,
	})
}

func TestInteractiveLoginPreservesPendingSMSWhenInputEnds(t *testing.T) {
	t.Parallel()

	raw := readObject(t, "../../../../tests/replay/fixtures/synthetic/http/auth-srp-sms-mfa.json")

	var exchanges []replay.Exchange

	decode(t, raw["exchanges"], &exchanges)

	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport),
		icloud.WithRandomSource(bytes.NewReader(nativeLoginEntropy(t, raw))))
	if err != nil {
		t.Fatal(err)
	}

	state := nativeCommandState(t, raw["initial_state"])
	nativeSourceParameters(t, raw, &state)
	savedPath := interactiveReplaySession(t, state)

	var output, diagnostic bytes.Buffer

	err = command.RunWithInput(t.Context(), client, []string{"auth", "login", sessionFlag, savedPath},
		io.NopCloser(strings.NewReader(expectedInventedPassword+"\n")),
		func(string) string { return "" }, &output, &diagnostic)
	if err == nil {
		t.Fatal("missing SMS code completed login")
	}

	data, err := os.ReadFile(filepath.Clean(savedPath))
	if err != nil {
		t.Fatal(err)
	}

	var saved icloud.NativeAuthState

	decode(t, data, &saved)

	if !saved.RequiresMFA || !saved.CodeRequested || saved.DeliveryMethod != icloud.TwoFactorDeliverySMS ||
		saved.Auth.SessionToken == nil || *saved.Auth.SessionToken != interactiveReplayToken {
		t.Fatal("interrupted interactive login discarded its pending SMS session")
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}

	assertNativeCommandPrivacy(t, output.String()+diagnostic.String(), []string{
		expectedInventedPassword, interactiveReplayToken, interactiveReplayCookie,
	})
}

func interactiveReplaySession(t *testing.T, state icloud.NativeAuthState) string {
	t.Helper()

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "private.json")

	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	return path
}
