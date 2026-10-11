package replay_test

import (
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestNativeLoginDefaultsReachAccountTokenExchange(t *testing.T) {
	t.Parallel()
	raw := authReplayObject(t, "fixtures/synthetic/http/auth-native-browser-defaults.json")

	var exchanges []replay.Exchange

	authReplayDecode(t, raw["exchanges"], &exchanges)

	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	var request icloud.AuthenticateRequest

	authReplayDecode(t, raw["request"], &request)

	result, err := client.Authenticate(t.Context(), request)
	if err != nil || result == nil || !result.Success {
		t.Fatalf("default native login did not complete: %v", err)
	}

	var expected []icloud.Header

	authReplayDecode(t, raw["expectedDefaultHeaders"], &expected)

	actual := make(map[string]string)

	for _, header := range result.State.Auth.Headers {
		actual[header.Name] = header.Value
	}

	for _, header := range expected {
		if actual[header.Name] != header.Value {
			t.Fatalf("native login lost persisted browser header %s", header.Name)
		}
	}

	if request.Auth.Headers != nil {
		t.Fatal("login changed caller-owned input")
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}
