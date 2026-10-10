package replay_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestNativePasswordLoginCommands(t *testing.T) {
	t.Parallel()

	for _, operation := range []string{"login", "renew"} {
		for _, stdin := range []bool{false, true} {
			source := "environment"
			if stdin {
				source = "stdin"
			}
			t.Run(operation+"/"+source, func(t *testing.T) {
				t.Parallel()
				nativePasswordLoginCommand(t, "auth-srp-paused-mfa", operation, stdin, false)
			})
		}
	}
}

func TestNativePasswordLoginRefusals(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"auth-srp-authorize-refused", "auth-srp-init-refused", "auth-srp-complete-refused"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			nativePasswordLoginCommand(t, name, "login", true, name == "auth-srp-complete-refused")
		})
	}
}

func nativePasswordLoginCommand(t *testing.T, name, operation string, stdin, adaptPause bool) {
	t.Helper()
	raw := readObject(t, filepath.Join("../../../../tests/replay/fixtures/synthetic/http", name+".json"))
	var exchanges []replay.Exchange
	decode(t, raw["exchanges"], &exchanges)
	if adaptPause {
		// Synthetic CLI control: the refusal fixture predates the explicit pause
		// flag. Use the exact schema-owned paused request from the paired Source
		// fixture, retaining this fixture's complete-refusal response boundary.
		paused := readObject(t, "../../../../tests/replay/fixtures/synthetic/http/auth-srp-paused-mfa.json")
		var pausedExchanges []replay.Exchange
		decode(t, paused["exchanges"], &pausedExchanges)
		exchanges[2].Request = pausedExchanges[2].Request
	}
	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}
	entropy := nativeLoginEntropy(t, raw)
	random := bytes.NewReader(entropy)
	client, err := icloud.New(icloud.WithHTTPTransport(transport), icloud.WithRandomSource(random))
	if err != nil {
		t.Fatal(err)
	}
	state := nativeCommandState(t, raw["initial_state"])
	nativeSourceParameters(t, raw, &state)
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "private.json")
	if err = os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{sessionFlag, path}
	if stdin {
		args = append(args, "--secret-stdin")
	}
	args = append(args, operation)

	environment := func(key string) string {
		if key == expectedAccountEnvironment {
			return state.AccountName
		}
		if key == expectedPasswordEnvironment {
			return "invented-password"
		}
		return ""
	}
	var output, diagnostic bytes.Buffer
	err = command.RunWithInput(t.Context(), client, args,
		io.NopCloser(strings.NewReader("invented-password\n")), environment, &output, &diagnostic)
	_, refused := raw["error"]
	if refused != (err != nil) {
		t.Fatalf("CLI password acceptance changed: %v", err)
	}
	if refused {
		var failure *icloud.ClientError
		if !errors.As(err, &failure) || failure.StatusCode() != exchanges[len(exchanges)-1].Response.Status {
			t.Fatalf("password failure lost typed HTTP boundary: %v", err)
		}
	}
	if consumedErr := transport.AssertConsumed(); consumedErr != nil {
		t.Fatal(consumedErr)
	}
	if random.Len() != 0 {
		t.Fatal("declared SRP entropy not consumed")
	}
	assertNativeLoginPersistence(t, path, raw, refused, encoded)
	assertNativeCommandPrivacy(t, output.String()+diagnostic.String(), []string{
		"invented-password", "synthetic-srp-token", "synthetic-srp-cookie", "synthetic-auth-value",
		"synthetic@example.invalid", `"responses"`, expectedAccountDataField})
}

func nativeLoginEntropy(t *testing.T, raw map[string]json.RawMessage) []byte {
	t.Helper()

	value, exists := raw["entropy"]
	if !exists {
		return []byte{}
	}

	var samples []string

	decode(t, readRawObject(t, value)["random_bytes"], &samples)
	if len(samples) != 1 {
		t.Fatal("unexpected SRP entropy inventory")
	}
	entropy, err := base64.StdEncoding.DecodeString(samples[0])
	if err != nil || len(entropy) != 256 {
		t.Fatal("invalid SRP entropy")
	}

	return entropy
}

func assertNativeLoginPersistence(t *testing.T, path string, raw map[string]json.RawMessage,
	refused bool, encoded []byte,
) {
	t.Helper()

	savedJSON, readErr := os.ReadFile(filepath.Clean(path))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if refused {
		if !bytes.Equal(savedJSON, encoded) {
			t.Fatal("failed password attempt overwrote prior private state")
		}

		return
	}

	var saved icloud.NativeAuthState

	decode(t, savedJSON, &saved)
	expected := readRawObject(t, readRawObject(t, raw["result"])["auth_state"])

	var account, actualAccount any

	decode(t, expected["account"], &account)
	decode(t, saved.AccountData, &actualAccount)
	if !reflect.DeepEqual(account, actualAccount) {
		t.Fatal("Source paused login account changed")
	}

	session := readRawObject(t, expected["session_data"])

	var token string

	decode(t, session["session_token"], &token)
	if saved.Auth.SessionToken == nil || *saved.Auth.SessionToken != token ||
		saved.Auth.AccountID != "synthetic-dsid" || saved.CodeRequested || saved.RequiresMFA {
		t.Fatal("Source paused login credentials or progress changed")
	}
}
