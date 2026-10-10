package replay_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

// The transport transcripts are implementation-derived Source evidence. Prompt-only
// fixtures add a synthetic CLI cancellation after the recorded prompt boundary.
func TestNativeBridgeCommands(t *testing.T) {
	paths, err := filepath.Glob("../../../../tests/replay/fixtures/synthetic/http/auth-bridge-*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 30 {
		t.Fatalf("bridge inventory changed: %d", len(paths))
	}
	for index, path := range paths {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".json"), func(t *testing.T) { runNativeBridgeCommand(t, path, index%2 == 0, false) })
	}
}

// This synthetic negative rotates credentials at a failed final trust boundary.
func TestNativeBridgeCommandTrustFailurePersistence(t *testing.T) {
	runNativeBridgeCommand(t, "../../../../tests/replay/fixtures/synthetic/http/auth-bridge-modern-code-204.json", true, true)
}

func TestNativeBridgeCommandEnvironmentCode(t *testing.T) {
	runNativeBridgeCommand(t, "../../../../tests/replay/fixtures/synthetic/http/auth-bridge-modern-code-204.json", false, false)
}

func TestNativeBridgeCommandStandardInputCode(t *testing.T) {
	runNativeBridgeCommand(t, "../../../../tests/replay/fixtures/synthetic/http/auth-bridge-modern-code-204.json", true, false)
}

type nativeBridgeCommandClient struct {
	icloud.Client
	runner *sdkBridgeReplay
	owner  *icloud.NativeBridgeSession
}

func (c *nativeBridgeCommandClient) OpenNativeBridgeSession(ctx context.Context, request icloud.OpenNativeBridgeSessionRequest, options ...icloud.NativeBridgeOption) (*icloud.NativeBridgeSession, error) {
	options = append(options, icloud.WithNativeBridgeDial(c.runner.dial))
	owner, err := c.Client.OpenNativeBridgeSession(ctx, request, options...)
	c.owner = owner
	return owner, err
}

type nativeBridgeCancelInput struct {
	cancel context.CancelFunc
	closed chan struct{}
	once   sync.Once
}

func (r *nativeBridgeCancelInput) Read([]byte) (int, error) { r.cancel(); <-r.closed; return 0, io.EOF }
func (r *nativeBridgeCancelInput) Close() error             { r.once.Do(func() { close(r.closed) }); return nil }

func runNativeBridgeCommand(t *testing.T, path string, stdin, lateFailure bool) {
	t.Helper()
	raw := readObject(t, path)
	var exchanges []replay.Exchange
	decode(t, raw["exchanges"], &exchanges)
	runner := &sdkBridgeReplay{t: t}
	decode(t, raw["bridge_network"], &runner.network)
	if lateFailure {
		exchanges = exchanges[:6]
		exchanges[5].Response = &replay.Response{Status: 503, Headers: []replay.Pair{{"Content-Type", "application/json"}, {"scnt", "synthetic-rotated-scnt"}, {"X-Apple-ID-Session-Id", "synthetic-rotated-session"}, {"X-Apple-Session-Token", "synthetic-rotated-token"}, {"X-Apple-TwoSV-Trust-Token", "synthetic-rotated-trust"}, {"Set-Cookie", "synthetic-rotated-cookie=synthetic-rotated-value; Path=/; Secure; HttpOnly"}}, Body: replay.Entity{Encoding: "base64", Value: json.RawMessage(`"e30="`)}}
		runner.network.Timeline = runner.network.Timeline[:len(runner.network.Timeline)-1]
	}
	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}
	runner.http = transport
	runner.entropy.replay = runner
	if value, ok := raw["entropy"]; ok {
		entropy := readRawObject(t, value)
		var random []string
		decode(t, entropy["random_bytes"], &random)
		if len(random) > 0 {
			sample, sampleErr := base64.StdEncoding.DecodeString(random[0])
			if sampleErr != nil {
				t.Fatal(sampleErr)
			}
			if len(sample) == 256 {
				runner.entropy.initial = sample
			}
		}
	}
	sdk, err := icloud.New(icloud.WithHTTPTransport(runner), icloud.WithRandomSource(&runner.entropy), icloud.WithClock(func() time.Time { return time.Unix(1700000000, 0) }))
	if err != nil {
		t.Fatal(err)
	}
	client := &nativeBridgeCommandClient{Client: sdk, runner: runner}
	state := nativeBridgeCommandState(t, raw)
	savedPath := filepath.Join(t.TempDir(), "private.json")
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(savedPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	var output, diagnostic bytes.Buffer
	environment := func(name string) string {
		switch name {
		case "GO_ICLOUD_CODE":
			return "123456"
		case "GO_ICLOUD_ACCOUNT":
			return state.AccountName
		case "GO_ICLOUD_PASSWORD":
			return "invented-password"
		default:
			return ""
		}
	}
	if strings.Contains(filepath.Base(path), "-full-") {
		// Source full-flow authentication prepares the saved state through the public
		// SDK. The command under test is mfa-bridge; CLI login's explicit pause flag
		// is covered separately and would change the canonical SRP request bytes.
		prepared, prepareErr := sdk.Authenticate(t.Context(), icloud.AuthenticateRequest{Auth: state.Auth, AccountName: state.AccountName, Password: "invented-password", SavedState: &state, TrustToken: state.TrustToken, AccountCountryCode: state.AccountCountryCode})
		if prepareErr != nil {
			t.Fatal(prepareErr)
		}
		preparedJSON, marshalErr := json.Marshal(prepared.State)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if writeErr := os.WriteFile(savedPath, preparedJSON, 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
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
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	arguments := []string{sessionFlag, savedPath}
	var input io.ReadCloser = io.NopCloser(strings.NewReader("123456\n"))
	if stdin || promptOnly {
		arguments = append(arguments, "--secret-stdin")
	}
	if promptOnly {
		input = &nativeBridgeCancelInput{cancel: cancel, closed: make(chan struct{})}
	}
	arguments = append(arguments, "mfa-bridge")
	err = command.RunWithInput(ctx, client, arguments, input, environment, &output, &diagnostic)
	if client.owner == nil {
		t.Fatalf("missing bridge owner: %v", err)
	}
	if strings.Contains(filepath.Base(path), "sms-fallback") {
		if err == nil {
			t.Fatal("failed bridge accepted before SMS fallback")
		}
		err = command.RunWithInput(t.Context(), client, []string{sessionFlag, savedPath, "--phone-id", "1", "mfa-request"}, io.NopCloser(strings.NewReader("")), environment, &output, &diagnostic)
	}
	_, expectedError := raw["error"]
	if expectedError {
		var failure *icloud.ClientError
		if !errors.As(err, &failure) {
			t.Fatalf("typed provider error lost: %v", err)
		}
	} else if promptOnly && !strings.Contains(filepath.Base(path), "sms-fallback") {
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("prompt cancellation lost: %v", err)
		}
	} else {
		accepted := !lateFailure
		if value, exists := raw["result"]; exists {
			var values []any
			result := readRawObject(t, value)
			if string(raw["operation"]) == `"flow"` {
				decode(t, result["value"], &values)
				if len(values) > 0 {
					if final, ok := values[len(values)-1].(bool); ok {
						accepted = final && !lateFailure
					}
				}
			}
		}
		if accepted != (err == nil) {
			t.Fatalf("CLI acceptance changed: accepted=%v error=%v", accepted, err)
		}
	}
	progress, stateErr := client.owner.State()
	if stateErr != nil {
		t.Fatal(stateErr)
	}
	if progress.Active {
		t.Fatal("command retained active bridge")
	}
	data, readErr := os.ReadFile(savedPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var saved icloud.NativeAuthState
	decode(t, data, &saved)
	if !strings.Contains(filepath.Base(path), "sms-fallback") {
		actual, _ := json.Marshal(saved)
		expected, _ := json.Marshal(progress.State)
		var actualValue, expectedValue any
		decode(t, actual, &actualValue)
		decode(t, expected, &expectedValue)
		if !reflect.DeepEqual(actualValue, expectedValue) {
			t.Fatal("private state differs from latest bridge credentials/progress")
		}
	}
	if lateFailure {
		if saved.Auth.SessionToken == nil || *saved.Auth.SessionToken != "synthetic-rotated-token" || saved.TrustToken != "synthetic-rotated-trust" || saved.CodeRequested || saved.RequiresMFA {
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
	console := output.String() + diagnostic.String()
	for _, secret := range []string{"123456", "synthetic-token", "synthetic-trust", "synthetic-cookie", "synthetic@example.invalid", "synthetic-rotated", `"responses"`, `"accountData"`} {
		if strings.Contains(console, secret) {
			t.Fatal("console disclosed private bridge data")
		}
	}
	if err = transport.AssertConsumed(); err != nil {
		t.Fatal(err)
	}
	if runner.timeline != len(runner.network.Timeline) {
		t.Fatalf("combined timeline consumed %d/%d", runner.timeline, len(runner.network.Timeline))
	}
	for _, socket := range runner.sockets {
		socket.mu.Lock()
		complete := socket.closed && socket.cursor == len(socket.events) && socket.failure == nil
		socket.mu.Unlock()
		if !complete {
			t.Fatal("socket transcript incomplete or left open")
		}
	}
	if runner.entropy.private != len(runner.network.PrivateScalars) || runner.entropy.prover != len(runner.network.ProverRandom) || runner.entropy.nonceReads != len(runner.network.Connections) || len(runner.entropy.initial) != 0 {
		t.Fatal("declared entropy not consumed")
	}
	if err = client.owner.Close(); err != nil {
		t.Fatal(err)
	}
}

func nativeBridgeCommandState(t *testing.T, raw map[string]json.RawMessage) icloud.NativeAuthState {
	t.Helper()
	state := nativeCommandState(t, raw["initial_state"])
	build, mastering := protocol.AuthClientBuildNumberValue, protocol.AuthClientMasteringNumberValue
	state.Auth.ClientBuildNumber = &build
	state.Auth.ClientMasteringNumber = &mastering
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
	expected := readRawObject(t, readRawObject(t, raw["result"])["auth_state"])
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
