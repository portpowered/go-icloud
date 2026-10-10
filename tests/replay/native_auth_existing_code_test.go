package replay_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func TestNativeExistingTrustedDeviceCodeReplay(t *testing.T) {
	t.Parallel()
	raw, transport, state := nativeFlowFixture(t, "auth-existing-trusted-device-code")
	state.CodeRequested = true
	before, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.UseExistingTrustedDeviceCode(t.Context(), icloud.NativeAuthRequest{
		Auth: state.Auth, State: state})
	if err != nil {
		t.Fatal(err)
	}

	nativeAssertState(t, raw, result)
	after, err := json.Marshal(state)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("existing-code selection mutated caller state")
	}
	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}
