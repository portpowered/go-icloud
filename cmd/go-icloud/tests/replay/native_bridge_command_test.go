package replay_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

// The transport transcripts are implementation-derived Source evidence. Prompt-only
// fixtures add a synthetic CLI cancellation after the recorded prompt boundary.
func TestNativeBridgeCommands(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("../../../../tests/replay/fixtures/synthetic/http/auth-bridge-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 30 {
		t.Fatalf("bridge inventory changed: %d", len(paths))
	}

	for index, path := range paths {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".json"), func(t *testing.T) {
			t.Parallel()
			runNativeBridgeCommand(t, path, index%2 == 0, false)
		})
	}
}

// This synthetic negative rotates credentials at a failed final trust boundary.
func TestNativeBridgeCommandTrustFailurePersistence(t *testing.T) {
	t.Parallel()

	runNativeBridgeCommand(t, expectedBridgeCodeFixture, true, true)
}

func TestNativeBridgeCommandEnvironmentCode(t *testing.T) {
	t.Parallel()

	runNativeBridgeCommand(t, expectedBridgeCodeFixture, false, false)
}

func TestNativeBridgeCommandStandardInputCode(t *testing.T) {
	t.Parallel()

	runNativeBridgeCommand(t, expectedBridgeCodeFixture, true, false)
}

type nativeBridgeCommandClient struct {
	icloud.Client

	runner *sdkBridgeReplay
	owner  *icloud.NativeBridgeSession
}

func (c *nativeBridgeCommandClient) OpenNativeBridgeSession(ctx context.Context,
	request icloud.OpenNativeBridgeSessionRequest, options ...icloud.NativeBridgeOption,
) (*icloud.NativeBridgeSession, error) {
	options = append(options, icloud.WithNativeBridgeDial(c.runner.dial))
	owner, err := c.Client.OpenNativeBridgeSession(ctx, request, options...)
	c.owner = owner

	if err != nil {
		return owner, fmt.Errorf("open replay bridge session: %w", err)
	}

	return owner, nil
}

type nativeBridgeCancelInput struct {
	cancel context.CancelFunc
	closed chan struct{}
	once   sync.Once
}

func (r *nativeBridgeCancelInput) Read([]byte) (int, error) {
	r.cancel()
	<-r.closed

	return 0, io.EOF
}
func (r *nativeBridgeCancelInput) Close() error {
	r.once.Do(func() { close(r.closed) })

	return nil
}

func runNativeBridgeCommand(t *testing.T, path string, stdin, lateFailure bool) {
	t.Helper()
	raw := readObject(t, path)
	runner, transport, sdk := nativeBridgeCommandReplay(t, raw, lateFailure)
	client := &nativeBridgeCommandClient{Client: sdk, runner: runner, owner: nil}
	state, savedPath := nativeBridgePrepareCommand(t, raw, path, sdk)

	var output, diagnostic bytes.Buffer

	environment := nativeBridgeCommandEnvironment(state)
	promptOnly := nativeBridgeCommandPromptOnly(t, raw)
	ctx, cancel := context.WithCancel(t.Context())

	defer cancel()

	arguments := []string{sessionFlag, savedPath}
	input := io.NopCloser(strings.NewReader("123456\n"))

	if stdin || promptOnly {
		arguments = append(arguments, "--secret-stdin")
	}

	if promptOnly {
		input = &nativeBridgeCancelInput{cancel: cancel, closed: make(chan struct{}), once: sync.Once{}}
	}

	arguments = append(arguments, "mfa-bridge")

	err := command.RunWithInput(ctx, client, arguments, input, environment, &output, &diagnostic)
	if client.owner == nil {
		t.Fatalf("missing bridge owner: %v", err)
	}

	if strings.Contains(filepath.Base(path), "sms-fallback") {
		if err == nil {
			t.Fatal("failed bridge accepted before SMS fallback")
		}

		err = command.RunWithInput(t.Context(), client,
			[]string{sessionFlag, savedPath, "--phone-id", "1", expectedMFARequestCommand},
			io.NopCloser(strings.NewReader("")), environment, &output, &diagnostic)
	}

	expectedError := nativeBridgeCommandAcceptance(t, raw, path, promptOnly, lateFailure, err)
	nativeBridgeCommandSavedState(t, raw, path, savedPath, client.owner, expectedError, lateFailure)

	console := output.String() + diagnostic.String()
	for _, secret := range []string{"123456", expectedSyntheticSessionValue, expectedSyntheticTrustValue, expectedSyntheticAuthCookie,
		expectedSyntheticAccountName, "synthetic-rotated", expectedReplayResponsesField, expectedAccountDataField} {
		if strings.Contains(console, secret) {
			t.Fatal("console disclosed private bridge data")
		}
	}

	nativeBridgeCommandConsumed(t, runner, transport)

	err = client.owner.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func nativeBridgeCommandReplay(t *testing.T, raw map[string]json.RawMessage,
	lateFailure bool,
) (*sdkBridgeReplay, *replay.HTTPTransport, *icloud.SDK) {
	t.Helper()

	var exchanges []replay.Exchange

	decode(t, raw["exchanges"], &exchanges)

	runner := new(sdkBridgeReplay)
	runner.t = t
	// Timeline events contain bounded fixture indexes. Decode this entire network
	// with the same JSON numeric representation as the events emitted below.
	networkErr := json.Unmarshal(raw["bridge_network"], &runner.network)
	if networkErr != nil {
		t.Fatal(networkErr)
	}

	if lateFailure {
		exchanges = exchanges[:6]
		exchanges[5].Response = &replay.Response{BodyRepresentation: "", Status: 503, Headers: []replay.Pair{
			{"Content-Type", "application/json"}, {"scnt", "synthetic-rotated-scnt"},
			{"X-Apple-ID-Session-Id", "synthetic-rotated-session"},
			{"X-Apple-Session-Token", expectedRotatedSessionToken},
			{"X-Apple-TwoSV-Trust-Token", expectedRotatedTrustToken},
			{expectedSetCookieHeader, "synthetic-rotated-cookie=synthetic-rotated-value; Path=/; Secure; HttpOnly"},
		}, Body: replay.Entity{Encoding: "base64", Value: json.RawMessage(`"e30="`),
			Matchers: nil, ContentTypePattern: "", Parts: nil}}
		runner.network.Timeline = runner.network.Timeline[:len(runner.network.Timeline)-1]
	}

	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}

	runner.http = transport
	runner.entropy.replay = runner
	nativeBridgeCommandEntropy(t, raw, runner)

	sdk, err := icloud.New(icloud.WithHTTPTransport(runner), icloud.WithRandomSource(&runner.entropy),
		icloud.WithClock(func() time.Time { return time.Unix(1700000000, 0) }))
	if err != nil {
		t.Fatal(err)
	}

	return runner, transport, sdk
}

func nativeBridgeCommandEntropy(t *testing.T, raw map[string]json.RawMessage, runner *sdkBridgeReplay) {
	t.Helper()

	value, ok := raw["entropy"]
	if !ok {
		return
	}

	entropy := readRawObject(t, value)

	var random []string

	decode(t, entropy["random_bytes"], &random)

	if len(random) == 0 {
		return
	}

	sample, sampleErr := base64.StdEncoding.DecodeString(random[0])
	if sampleErr != nil {
		t.Fatal(sampleErr)
	}

	if len(sample) == 256 {
		runner.entropy.initial = sample
	}
}

func nativeBridgePrepareCommand(t *testing.T, raw map[string]json.RawMessage,
	path string, sdk *icloud.SDK,
) (icloud.NativeAuthState, string) {
	t.Helper()
	state := nativeBridgeCommandState(t, raw)
	savedPath := filepath.Join(t.TempDir(), "private.json")

	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(savedPath, encoded, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(filepath.Base(path), "-full-") {
		// Source full-flow authentication prepares the saved state through the public
		// SDK. The command under test is mfa-bridge; CLI login's explicit pause flag
		// is covered separately and would change the canonical SRP request bytes.
		prepared, prepareErr := sdk.Authenticate(t.Context(), icloud.AuthenticateRequest{
			Auth: state.Auth, AccountName: state.AccountName, Password: expectedInventedPassword, SavedState: &state,
			TrustToken: state.TrustToken, AccountCountryCode: state.AccountCountryCode,
			AcceptTerms: false, ForceRefresh: false, PauseTwoFactor: false, Service: nil,
		})
		if prepareErr != nil {
			t.Fatal(prepareErr)
		}

		preparedJSON, marshalErr := json.Marshal(prepared.State)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}

		writeErr := os.WriteFile(savedPath, preparedJSON, 0o600)
		if writeErr != nil {
			t.Fatal(writeErr)
		}
	}

	return state, savedPath
}

func nativeBridgeCommandEnvironment(state icloud.NativeAuthState) func(string) string {
	return func(name string) string {
		switch name {
		case "GO_ICLOUD_CODE":
			return "123456"
		case expectedAccountEnvironment:
			return state.AccountName
		case expectedPasswordEnvironment:
			return expectedInventedPassword
		default:
			return ""
		}
	}
}

func nativeBridgeCommandPromptOnly(t *testing.T, raw map[string]json.RawMessage) bool {
	t.Helper()

	promptOnly := true

	var actions []struct {
		Operation string `json:"operation"`
	}

	if string(raw["operation"]) == `"flow"` {
		decode(t, raw["inputs"], &actions)

		for _, action := range actions {
			if action.Operation == "validate_2fa_code" {
				promptOnly = false
			}
		}
	}

	return promptOnly
}

func nativeBridgeCommandAcceptance(t *testing.T, raw map[string]json.RawMessage,
	path string, promptOnly, lateFailure bool, err error,
) bool {
	t.Helper()

	_, expectedError := raw["error"]

	switch {
	case expectedError:
		var failure *icloud.ClientError
		if !errors.As(err, &failure) {
			t.Fatalf("typed provider error lost: %v", err)
		}
	case promptOnly && !strings.Contains(filepath.Base(path), "sms-fallback"):
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("prompt cancellation lost: %v", err)
		}
	default:
		accepted := nativeBridgeCommandExpectedAcceptance(t, raw, lateFailure)
		if accepted != (err == nil) {
			t.Fatalf("CLI acceptance changed: accepted=%v error=%v", accepted, err)
		}
	}

	return expectedError
}

func nativeBridgeCommandExpectedAcceptance(t *testing.T, raw map[string]json.RawMessage, lateFailure bool) bool {
	t.Helper()

	value, exists := raw["result"]
	if !exists || string(raw["operation"]) != `"flow"` {
		return !lateFailure
	}

	result := readRawObject(t, value)

	var values []any

	decode(t, result["value"], &values)

	if len(values) == 0 {
		return !lateFailure
	}

	final, ok := values[len(values)-1].(bool)
	if !ok {
		return !lateFailure
	}

	return final && !lateFailure
}

func nativeBridgeCommandSavedState(t *testing.T, raw map[string]json.RawMessage,
	path, savedPath string, owner *icloud.NativeBridgeSession, expectedError, lateFailure bool,
) {
	t.Helper()

	progress, stateErr := owner.State()
	if stateErr != nil {
		t.Fatal(stateErr)
	}

	if progress.Active {
		t.Fatal("command retained active bridge")
	}

	data, readErr := os.ReadFile(filepath.Clean(savedPath))
	if readErr != nil {
		t.Fatal(readErr)
	}

	var saved icloud.NativeAuthState

	decode(t, data, &saved)

	if !strings.Contains(filepath.Base(path), "sms-fallback") {
		actual, actualErr := json.Marshal(saved)
		if actualErr != nil {
			t.Fatal(actualErr)
		}

		expected, expectedErr := json.Marshal(progress.State)
		if expectedErr != nil {
			t.Fatal(expectedErr)
		}

		var actualValue, expectedValue any

		decode(t, actual, &actualValue)
		decode(t, expected, &expectedValue)

		if !reflect.DeepEqual(actualValue, expectedValue) {
			t.Fatal("private state differs from latest bridge credentials/progress")
		}
	}

	if lateFailure {
		if saved.Auth.SessionToken == nil || *saved.Auth.SessionToken != expectedRotatedSessionToken ||
			saved.TrustToken != expectedRotatedTrustToken || saved.CodeRequested || saved.RequiresMFA {
			t.Fatal("failed trust lost rotated tokens or completion progress")
		}

		found := false

		for _, cookie := range saved.Auth.Cookies {
			if cookie.Name == "synthetic-rotated-cookie" && cookie.Value == "synthetic-rotated-value" {
				found = true
			}
		}

		if !found {
			t.Fatal("failed trust lost rotated cookie")
		}
	}

	if !expectedError && !lateFailure {
		nativeBridgeCommandSourceState(t, raw, saved)
	}
}

func nativeBridgeCommandConsumed(t *testing.T, runner *sdkBridgeReplay, transport *replay.HTTPTransport) {
	t.Helper()

	err := transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}

	runner.mu.Lock()
	timeline := runner.timeline
	sockets := append([]*sdkBridgeSocket(nil), runner.sockets...)
	runner.mu.Unlock()

	if timeline != len(runner.network.Timeline) {
		t.Fatalf("combined timeline consumed %d/%d", timeline, len(runner.network.Timeline))
	}

	for _, socket := range sockets {
		socket.mu.Lock()
		complete := socket.closed && socket.cursor == len(socket.events) && socket.failure == nil
		socket.mu.Unlock()

		if !complete {
			t.Fatal("socket transcript incomplete or left open")
		}
	}

	runner.entropy.mu.Lock()
	entropyComplete := runner.entropy.private == len(runner.network.PrivateScalars) &&
		runner.entropy.prover == len(runner.network.ProverRandom) &&
		runner.entropy.nonceReads == len(runner.network.Connections) && len(runner.entropy.initial) == 0
	runner.entropy.mu.Unlock()

	if !entropyComplete {
		t.Fatal("declared entropy not consumed")
	}
}

func nativeBridgeCommandState(t *testing.T, raw map[string]json.RawMessage) icloud.NativeAuthState {
	t.Helper()
	state := nativeCommandState(t, raw["initial_state"])
	nativeSourceParameters(t, raw, &state)

	initial := readRawObject(t, raw["initial_state"])
	if value, exists := initial["requires_mfa"]; exists {
		decode(t, value, &state.RequiresMFA)
	}

	challenge := readRawObject(t, initial["auth_data"])
	state.Challenge.ProviderData = bytes.Clone(initial["auth_data"])

	state.Challenge.BridgeBootstrap = []byte(`{"twoSV":` + string(initial["auth_data"]) + `}`)

	if value, exists := challenge["authInitialRoute"]; exists {
		decode(t, value, &state.Challenge.AuthInitialRoute)
	}

	if value, exists := challenge["hasTrustedDevices"]; exists {
		decode(t, value, &state.Challenge.HasTrustedDevices)
	}

	if value, exists := challenge["authFactors"]; exists {
		decode(t, value, &state.Challenge.AuthFactors)
	}

	var account struct {
		Webservices map[string]map[string]string `json:"webservices"`
	}

	decode(t, initial["account_data"], &account)
	state.Auth.DriveServiceURL = account.Webservices["drivews"]["url"]
	state.Auth.FindMyServiceURL = account.Webservices["findme"]["url"]

	for index := range state.Auth.Cookies {
		state.Auth.Cookies[index].HTTPOnly = true
	}

	return state
}

func nativeBridgeCommandSourceState(t *testing.T, raw map[string]json.RawMessage, state icloud.NativeAuthState) {
	t.Helper()
	expected := readRawObject(t, readRawObject(t, raw["result"])[expectedAuthStateKey])

	var account, actualAccount any

	decode(t, expected["account"], &account)
	decode(t, state.AccountData, &actualAccount)

	if !reflect.DeepEqual(account, actualAccount) {
		t.Fatal("Source account data changed")
	}

	session := readRawObject(t, expected["session_data"])
	if value, ok := session["session_token"]; ok {
		var token string

		decode(t, value, &token)

		if state.Auth.SessionToken == nil || *state.Auth.SessionToken != token {
			t.Fatal("Source session token changed")
		}
	}

	if value, ok := session["trust_token"]; ok {
		var token string

		decode(t, value, &token)

		if state.TrustToken != token {
			t.Fatal("Source trust token changed")
		}
	}

	for _, field := range []struct {
		name   string
		actual bool
	}{{"code_requested", state.CodeRequested}, {"requires_mfa", state.RequiresMFA}} {
		if value, exists := expected[field.name]; exists {
			var expectedFlag bool

			decode(t, value, &expectedFlag)

			if expectedFlag != field.actual {
				t.Fatalf("Source %s changed", field.name)
			}
		}
	}

	var provider, actualProvider any

	decode(t, expected["challenge"], &provider)

	if len(state.Challenge.ProviderData) == 0 {
		actualProvider = map[string]any{}
	} else {
		decode(t, state.Challenge.ProviderData, &actualProvider)
	}

	if !reflect.DeepEqual(provider, actualProvider) {
		t.Fatal("Source challenge payload changed")
	}
}
