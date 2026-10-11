package replay_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const interactiveSecurityKeyCommand = "mfa-security-key"

func TestInteractiveLoginReportsUnsupportedMFARoutes(t *testing.T) {
	t.Parallel()

	for _, test := range []struct{ fixture, instruction string }{
		{fixture: "auth-srp-security-key-delivery", instruction: interactiveSecurityKeyCommand},
		{fixture: "auth-srp-non-sms-delivery", instruction: "no SMS route"},
	} {
		t.Run(test.fixture, func(t *testing.T) {
			t.Parallel()

			raw := readObject(t, filepath.Join("../../../../tests/replay/fixtures/synthetic/http", test.fixture+".json"))

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
			if err == nil || !strings.Contains(err.Error(), test.instruction) {
				t.Fatalf("provider route lost its continuation instruction: %v", err)
			}

			data, err := os.ReadFile(filepath.Clean(savedPath))
			if err != nil {
				t.Fatal(err)
			}

			var saved icloud.NativeAuthState

			decode(t, data, &saved)

			if !saved.RequiresMFA || saved.Auth.ClientID != state.Auth.ClientID {
				t.Fatal("unsupported route discarded its pending authentication session")
			}

			err = transport.AssertConsumed()
			if err != nil {
				t.Fatal(err)
			}

			assertNativeCommandPrivacy(t, output.String()+diagnostic.String(), []string{expectedInventedPassword})
		})
	}
}
