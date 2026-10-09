package replay_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func TestNativeTermsTrustPreservesSourceDeliveryState(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"auth-terms-trust-accepted-notice", "auth-terms-trust-untrusted-notice"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			raw, transport, state := nativeFlowFixture(t, name)
			nativeFixtureMFAState(t, raw, &state)
			state.AcceptTerms = true
			notice := "Synthetic retained delivery notice"
			state.DeliveryNotice = &notice
			before, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			client, err := icloud.New(icloud.WithHTTPTransport(transport))
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.TrustSession(t.Context(), icloud.NativeAuthRequest{Auth: state.Auth, State: state})
			if err != nil {
				t.Fatal(err)
			}
			nativeAssertState(t, raw, result)
			expected := authReplayObjectBytes(t, raw["result"])
			var values []json.RawMessage
			authReplayDecode(t, expected["value"], &values)
			var accepted bool
			authReplayDecode(t, values[1], &accepted)
			if result.Success != accepted || !result.State.AcceptTerms {
				t.Fatal("terms trust lost acceptance policy or final trust verdict")
			}
			if accepted && result.State.DeliveryNotice != nil {
				t.Fatal("accepted terms trust retained old notice")
			}
			after, err := json.Marshal(state)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("terms trust mutated caller state")
			}
			if err = transport.AssertConsumed(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
