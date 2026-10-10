package replay_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func TestNativeAuthenticationChallengeReplay(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"auth-challenge-direct-sms", "auth-challenge-direct-refused"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			raw, transport, state := nativeFlowFixture(t, name)

			client, err := icloud.New(icloud.WithHTTPTransport(transport))
			if err != nil {
				t.Fatal(err)
			}

			before, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}

			result, err := client.GetAuthenticationChallenge(t.Context(), icloud.NativeAuthRequest{
				Auth: state.Auth, State: state})
			nativeFlowExpectedError(t, raw, err)

			if err == nil {
				expected := authReplayObjectBytes(t, raw["result"])
				if !reflect.DeepEqual(accountJSON(t, result.State.Challenge.ProviderData),
					accountJSON(t, expected["value"])) {
					t.Fatal("direct challenge lost Source MFA options")
				}

				if result.State.Challenge.Mode != "sms" || len(result.State.Challenge.PhoneNumbers) != 1 ||
					result.State.Challenge.PhoneNumbers[0].Number != "synthetic-masked" ||
					result.State.CodeRequested || result.State.DeliveryMethod != icloud.TwoFactorDeliveryUnknown {
					t.Fatal("direct challenge lost typed phone choices or delivery reset")
				}

				nativeFlowResponses(t, raw, result.Responses)
			}

			after, marshalErr := json.Marshal(state)
			if marshalErr != nil || !bytes.Equal(before, after) {
				t.Fatal("direct challenge changed caller-owned state")
			}

			consumedErr := transport.AssertConsumed()
			if consumedErr != nil {
				t.Fatal(consumedErr)
			}
		})
	}
}
