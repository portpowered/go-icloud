package replay_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func TestNativeTrustedDevicesReplay(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"auth-trusted-devices-0", "auth-trusted-devices-1", "auth-trusted-devices-3"} {
		t.Run(name, func(t *testing.T) { t.Parallel(); nativeDevicesReplay(t, name) })
	}
}

func nativeDevicesReplay(t *testing.T, name string) {
	t.Helper()
	raw, transport, state := nativeFlowFixture(t, name)
	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.ListTrustedDevices(t.Context(), icloud.NativeAuthRequest{Auth: state.Auth, State: state})
	if err != nil {
		t.Fatal(err)
	}
	expected := authReplayObjectBytes(t, raw["result"])
	var devices []json.RawMessage

	authReplayDecode(t, expected["value"], &devices)
	if len(result.Devices) != len(devices) {
		t.Fatal("trusted-device count differs")
	}
	for index, device := range devices {
		if !reflect.DeepEqual(accountJSON(t, result.Devices[index].Metadata), accountJSON(t, device)) {
			t.Fatal("trusted-device metadata differs")
		}
	}

	nativeFlowResponses(t, raw, result.Responses)
	if err = transport.AssertConsumed(); err != nil {
		t.Fatal(err)
	}
}
