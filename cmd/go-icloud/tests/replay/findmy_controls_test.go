package replay_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestFindMySoundCommands(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"findmy-sound-success", "findmy-sound-success-204",
		"findmy-sound-ack-invalid-json-200", "findmy-sound-ack-refused-200-json", "findmy-sound-unavailable"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			raw := readObject(t, filepath.Join("../../../../tests/replay/fixtures/synthetic/http", name+".json"))

			var inputs []string

			decode(t, raw["inputs"], &inputs)
			runFindMyControl(t, raw, []string{"--subject", inputs[0], "findmy-sound"}, false)
		})
	}
}

func TestFindMyDeviceLookupFailure(t *testing.T) {
	t.Parallel()
	raw := readObject(t, "../../../../tests/replay/fixtures/synthetic/http/findmy-sound-success.json")

	var exchanges []replay.Exchange

	decode(t, raw["exchanges"], &exchanges)

	encoded, err := json.Marshal(exchanges[:1])
	if err != nil {
		t.Fatal(err)
	}

	raw["exchanges"] = encoded
	runFindMyControl(t, raw, []string{"--device", "unknown-device", "findmy-device"}, true)
}

func runFindMyControl(t *testing.T, raw map[string]json.RawMessage, arguments []string, lookupFailure bool) {
	t.Helper()

	var exchanges []replay.Exchange

	decode(t, raw["exchanges"], &exchanges)

	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	path := writeControlSession(t, fixtureAuth(t, raw["initial_state"]))
	args := make([]string, 0, 4+len(arguments))
	args = append(args, sessionFlag, path, "--device", "synthetic-device-0")
	args = append(args, arguments...)

	var output, diagnostic bytes.Buffer

	err = command.Run(t.Context(), client, args, &output, &diagnostic)
	assertControlOutcome(t, raw, &output, err, lookupFailure)

	if diagnostic.Len() != 0 || strings.Contains(output.String(), "responses") {
		t.Fatal("CLI disclosed authentication metadata or unexpected diagnostics")
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func assertControlOutcome(t *testing.T, raw map[string]json.RawMessage,
	output *bytes.Buffer, err error, lookupFailure bool,
) {
	t.Helper()

	if _, failed := raw["error"]; failed || lookupFailure {
		var failure *icloud.ClientError
		if !errors.As(err, &failure) || output.Len() != 0 {
			t.Fatalf("CLI lost Find My failure: %v", err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
}

func writeControlSession(t *testing.T, auth icloud.AuthContext) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.json")
	//nolint:gosec // Only deterministic synthetic fixture credentials are serialized into temporary test files.
	data, err := json.Marshal(auth)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	return path
}
