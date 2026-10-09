package replay_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestNativeSRPRejectsInvalidChallengeWithResponseEvidence(t *testing.T) {
	t.Parallel()
	for _, control := range []string{"zero server", "empty salt", "empty context", "zero iterations", "unknown protocol"} {
		t.Run(control, func(t *testing.T) {
			t.Parallel()
			raw, _, state := nativeFlowFixture(t, "auth-srp-s2k")
			var exchanges []replay.Exchange
			authReplayDecode(t, raw["exchanges"], &exchanges)
			exchanges = exchanges[:2]
			original := nativeFlowBody(t, exchanges[1].Response.Body)
			var challenge auth.AuthSRPInitResponse
			authReplayDecode(t, original, &challenge)
			switch control {
			case "zero server":
				challenge.B = []byte{0}
			case "empty salt":
				challenge.Salt = nil
			case "empty context":
				challenge.C = ""
			case "zero iterations":
				challenge.Iteration = 0
			case "unknown protocol":
				challenge.Protocol = auth.AuthSRPProtocol("unsupported")
			}
			body, err := json.Marshal(challenge)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(base64.StdEncoding.EncodeToString(body))
			if err != nil {
				t.Fatal(err)
			}
			exchanges[1].Response.Body.Value = encoded
			transport, err := replay.NewHTTPTransport(exchanges)
			if err != nil {
				t.Fatal(err)
			}
			random := nativeFixtureEntropy(t, raw)
			client, err := icloud.New(icloud.WithHTTPTransport(transport), icloud.WithRandomSource(bytes.NewReader(random)))
			if err != nil {
				t.Fatal(err)
			}
			initial := authReplayObjectBytes(t, raw["initial_state"])
			var password string
			authReplayDecode(t, initial["synthetic_password"], &password)
			_, err = client.Authenticate(t.Context(), icloud.AuthenticateRequest{Auth: state.Auth, AccountName: state.AccountName,
				Password: password, TrustToken: state.TrustToken, AccountCountryCode: state.AccountCountryCode, ForceRefresh: true,
				PauseTwoFactor: false, Service: nil, SavedState: &state, AcceptTerms: false})
			var failure *icloud.ClientError
			if !errors.As(err, &failure) || failure.Kind() != icloud.InvalidResponse || failure.StatusCode() != exchanges[1].Response.Status || !bytes.Equal(failure.ResponseBody(), body) || len(failure.PriorResponses()) != 1 {
				t.Fatal("invalid SRP challenge lost typed error or response evidence")
			}
			if err = transport.AssertConsumed(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNativeMissingPasswordRetainsRejectedSavedSessionEvidence(t *testing.T) {
	t.Parallel()
	raw, _, state := nativeFlowFixture(t, "auth-authenticate-stale-token")
	var exchanges []replay.Exchange
	authReplayDecode(t, raw["exchanges"], &exchanges)
	exchanges[1].Response.Status = 503
	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}
	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Authenticate(t.Context(), icloud.AuthenticateRequest{Auth: state.Auth,
		AccountName: state.AccountName, Password: "", TrustToken: state.TrustToken,
		AccountCountryCode: state.AccountCountryCode, SavedState: &state, ForceRefresh: false,
		PauseTwoFactor: false, Service: nil, AcceptTerms: false})
	var failure *icloud.ClientError
	if !errors.As(err, &failure) || failure.Kind() != icloud.Unauthorized {
		t.Fatal("missing credentials after rejected saved session lost terminal classification")
	}
	prior := failure.PriorResponses()
	if len(prior) != len(exchanges) {
		t.Fatal("missing credentials discarded rejected saved-session evidence")
	}
	for index, exchange := range exchanges {
		if prior[index].StatusCode != exchange.Response.Status || len(prior[index].Headers) != len(exchange.Response.Headers) {
			t.Fatal("saved-session failure evidence changed status or headers")
		}
	}
	if err = transport.AssertConsumed(); err != nil {
		t.Fatal(err)
	}
}

func nativeFixtureEntropy(t *testing.T, raw map[string]json.RawMessage) []byte {
	t.Helper()
	var entropy struct {
		RandomBytes []string `json:"random_bytes"`
	}
	authReplayDecode(t, raw["entropy"], &entropy)
	result := []byte{}
	for _, value := range entropy.RandomBytes {
		decoded, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, decoded...)
	}
	return result
}
