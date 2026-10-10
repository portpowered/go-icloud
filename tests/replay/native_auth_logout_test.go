package replay_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func TestNativeLogoutReplay(t *testing.T) {
	t.Parallel()

	for _, scenario := range []string{nativeLogoutDefaultFixture, "auth-logout-all_sessions", "auth-logout-keep_trusted",
		"auth-logout-clear_local_session", "auth-logout-no-cookie", "auth-logout-remote-error",
		"auth-logout-remote-refused"} {
		t.Run(scenario, func(t *testing.T) { t.Parallel(); nativeLogoutReplay(t, scenario) })
	}
}

type nativeLogoutCancelTransport struct {
	cancel context.CancelFunc
}

func (transport nativeLogoutCancelTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.cancel()
	return nil, fmt.Errorf("synthetic logout cancellation: %w", request.Context().Err())
}

func TestNativeLogoutCancellation(t *testing.T) {
	t.Parallel()

	_, _, state := nativeFlowFixture(t, nativeLogoutDefaultFixture)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	client, err := icloud.New(icloud.WithHTTPTransport(nativeLogoutCancelTransport{cancel: cancel}))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Logout(ctx, icloud.LogoutRequest{Auth: state.Auth, State: state, KeepTrusted: false,
		AllSessions: false, PreserveLocalSession: false})

	var failure *icloud.ClientError

	if result != nil || !errors.As(err, &failure) || failure.Kind() != icloud.Canceled ||
		!errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled logout cleared local state or lost cancellation: %v", err)
	}
}

func nativeLogoutReplay(t *testing.T, name string) {
	t.Helper()

	raw, transport, state := nativeFlowFixture(t, name)

	var keywords map[string]bool

	authReplayDecode(t, raw["keyword_inputs"], &keywords)
	preserve, exists := keywords["clear_local_session"]
	request := icloud.LogoutRequest{Auth: state.Auth, State: state, KeepTrusted: keywords["keep_trusted"],
		AllSessions: keywords["all_sessions"], PreserveLocalSession: exists && !preserve}
	before, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Logout(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}

	assertNativeLogoutResult(t, raw, state, result)

	after, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("logout mutated caller state")
	}

	nativeFlowResponses(t, raw, result.Responses)
	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func assertNativeLogoutResult(t *testing.T, raw map[string]json.RawMessage, state icloud.NativeAuthState,
	result *icloud.LogoutResult,
) {
	t.Helper()

	expected := authReplayObjectBytes(t, raw["result"])
	value := authReplayObjectBytes(t, expected["value"])

	var confirmed, cleared bool

	authReplayDecode(t, value["remote_logout_confirmed"], &confirmed)
	authReplayDecode(t, value["local_session_cleared"], &cleared)
	if result.RemoteConfirmed != confirmed || result.LocalCleared != cleared {
		t.Fatal("logout confirmation mismatch")
	}
	if cleared {
		assertNativeLogoutCredentials(t, result.State)
		assertNativeLogoutProgress(t, result.State)
	} else if result.State.Auth.AccountID != state.Auth.AccountID {
		t.Fatal("logout lost preserved identity")
	}
}

func assertNativeLogoutCredentials(t *testing.T, state icloud.NativeAuthState) {
	t.Helper()

	if state.Auth.AccountID != "" || state.Auth.SessionToken != nil || len(state.Auth.Cookies) != 0 ||
		state.TrustToken != "" {
		t.Fatal("logout retained authentication-derived credentials")
	}
	services := []string{state.Auth.PhotosServiceURL, state.Auth.DriveServiceURL, state.Auth.FindMyServiceURL,
		state.Auth.AccountServiceURL, state.Auth.RemindersServiceURL, state.Auth.DriveDocumentServiceURL}

	for _, service := range services {
		if service != "" {
			t.Fatal("logout retained discovered service access")
		}
	}
}

func assertNativeLogoutProgress(t *testing.T, state icloud.NativeAuthState) {
	t.Helper()

	if state.RequiresMFA || state.CodeRequested || len(state.Challenge.BridgeBootstrap) != 0 ||
		state.Challenge.SecurityKeyChallenge != nil || state.DeliveryMethod != icloud.TwoFactorDeliveryUnknown {
		t.Fatal("logout retained pending authentication progress")
	}
}
