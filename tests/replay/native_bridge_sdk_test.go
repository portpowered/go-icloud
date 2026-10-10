package replay_test

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha1" //nolint:gosec // RFC 6455 requires SHA-1 for the upgrade accept digest.
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/bridge"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/bridgepb"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
	"google.golang.org/protobuf/proto"
)

var errUndeclaredSRPEntropy = errors.New("undeclared SRP entropy")
var errUnboundedOptionalSignatureEntropy = errors.New("unbounded optional signature entropy")
var errUnexpectedOptionalECDSAEntropy = errors.New("unexpected optional ECDSA entropy")
var errInvalidFixtureNonce = errors.New("invalid fixture nonce")
var errUndeclaredWebsocketOrUUIDEntropy = errors.New("undeclared websocket or UUID entropy")
var errSecureDialDeadlineDiffers = errors.New("secure dial deadline differs")
var errUndeclaredBridgeConnection = errors.New("undeclared bridge connection")
var errUnexpectedSecureDialTarget = errors.New("unexpected secure dial target")
var errUnboundedSignatureEntropy = errors.New("unbounded signature entropy")
var errUndeclaredBootstrapScalar = errors.New("undeclared bootstrap scalar")
var errUndeclaredProverScalar = errors.New("undeclared prover scalar")
var errIncorrectProverBound = errors.New("incorrect prover bound")
var errInvalidFixtureScalar = errors.New("invalid fixture scalar")
var errInvalidUpgradeTarget = errors.New("invalid upgrade target")
var errWebsocketEntropyDiffers = errors.New("websocket entropy differs")
var errFrameMaskEntropyDiffers = errors.New("frame mask entropy differs")
var errUndeclaredEntropyLength = errors.New("undeclared entropy length")

type sdkBridgeNetwork struct {
	PrivateScalars []string `json:"private_scalars"` //nolint:tagliatelle // Preserve the pinned Python fixture field.
	ProverRandom   []struct {
		Upper string `json:"upper_hex"` //nolint:tagliatelle // Preserve the pinned Python fixture field.
		Value string `json:"value_hex"` //nolint:tagliatelle // Preserve the pinned Python fixture field.
	} `json:"prover_random"` //nolint:tagliatelle // Preserve the pinned Python fixture field.
	Connections []struct {
		Connection bridgeSocketConnection `json:"connection"`
		Bootstrap  struct {
			Public     string `json:"public_key"` //nolint:tagliatelle // Preserve the pinned Python fixture field.
			Nonce      string `json:"nonce"`
			Expiration uint32 `json:"expiration_seconds"` //nolint:tagliatelle // Preserve the pinned Python fixture field.
		} `json:"bootstrap"`
		Events []bridgeSocketEvent `json:"events"`
	} `json:"connections"`
	Timeline []map[string]any `json:"timeline"`
}

type sdkBridgeReplay struct {
	t                                    *testing.T
	network                              sdkBridgeNetwork
	http                                 *replay.HTTPTransport
	mu                                   sync.Mutex
	timeline, httpIndex, connectionIndex int
	sockets                              []*sdkBridgeSocket
	entropy                              sdkBridgeEntropy
	session                              *icloud.NativeBridgeSession
	overlap, overlapChecked              bool
}

type sdkBridgeEntropy struct {
	replay                                           *sdkBridgeReplay
	initial                                          []byte
	private, prover                                  int
	signatureReads, nonceReads, wideReads, maskReads int
	signing, uuid                                    bool
}

func (r *sdkBridgeEntropy) Read(destination []byte) (int, error) {
	clear(destination)

	switch len(destination) {
	case 256:
		if len(r.initial) != 256 {
			return 0, errUndeclaredSRPEntropy
		}
		copy(destination, r.initial)
		r.initial = nil
	case 1:
		r.signatureReads++
		if r.signatureReads > 4 {
			return 0, errUnboundedOptionalSignatureEntropy
		}
		if !r.signing {
			return 0, errUnexpectedOptionalECDSAEntropy
		}
	case 8:
		r.nonceReads++
		r.uuid = false
		r.signatureReads = 0
		index := r.replay.connectionIndex
		if index >= len(r.replay.network.Connections) {
			return 0, io.EOF
		}
		nonce, err := base64.StdEncoding.DecodeString(r.replay.network.Connections[index].Bootstrap.Nonce)
		if err != nil || len(nonce) != 17 {
			return 0, errInvalidFixtureNonce
		}
		copy(destination, nonce[9:])
		r.signing = true
	case 16:
		r.wideReads++
		if r.wideReads > len(r.replay.network.Connections)+1 {
			return 0, errUndeclaredWebsocketOrUUIDEntropy
		}
		if r.uuid {
			destination[3] = 1
			r.uuid = false
		} else {
			for index := range destination {
				destination[index] = byte(index)
			}
			r.uuid = true
		}
	case 4:
		r.maskReads++
		for index := range destination {
			destination[index] = byte(index)
		}
	case 32:
		return r.readScalar(destination)
	default:
		return 0, fmt.Errorf("%w %d", errUndeclaredEntropyLength, len(destination))
	}
	return len(destination), nil
}

func (r *sdkBridgeReplay) RoundTrip(request *http.Request) (*http.Response, error) {
	if r.overlap && !r.overlapChecked && strings.HasSuffix(request.URL.Path, "/bridge/step/2") {
		r.overlapChecked = true
		_, err := r.session.VerifyCode(r.t.Context(), icloud.VerifyNativeBridgeCodeRequest{Code: "123456"})
		var failure *icloud.ClientError
		if !errors.Is(err, bridge.ErrBusy) || !errors.As(err, &failure) || failure.Kind() != icloud.Busy {
			r.t.Fatalf("overlap guard lost: %v", err)
		}
		state, stateErr := r.session.State()
		if stateErr != nil {
			r.t.Fatal(stateErr)
		}
		if !state.Active {
			r.t.Fatal("overlap closed active verification")
		}
	}

	r.take(map[string]any{"surface": "http", "exchange": float64(r.httpIndex)})
	r.httpIndex++
	response, err := r.http.RoundTrip(request)
	if err != nil {
		if request.GetBody != nil {
			body, _ := request.GetBody()
			data, _ := io.ReadAll(body)
			_ = body.Close()

			r.t.Logf("HTTP mismatch %s body=%s: %v", request.URL, data, err)
		}
	}
	return response, err
}

func (r *sdkBridgeReplay) take(event map[string]any) {
	r.t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.timeline >= len(r.network.Timeline) {
		r.t.Fatalf("extra combined timeline event %v", event)
	}
	if !reflect.DeepEqual(event, r.network.Timeline[r.timeline]) {
		r.t.Fatalf("combined timeline[%d] = %v; expected %v", r.timeline, event, r.network.Timeline[r.timeline])
	}
	r.timeline++
}

func (r *sdkBridgeReplay) dial(ctx context.Context, network, address string) (net.Conn, error) {
	deadline, bounded := ctx.Deadline()
	if !bounded || time.Until(deadline) <= 0 || time.Until(deadline) > 30*time.Second {
		return nil, errSecureDialDeadlineDiffers
	}
	index := r.connectionIndex
	if index >= len(r.network.Connections) {
		return nil, errUndeclaredBridgeConnection
	}
	expected := r.network.Connections[index]
	if network != "tcp" || address != "bridge.example.invalid:443" {
		return nil, errUnexpectedSecureDialTarget
	}
	script := new(bridgeScriptSocket)
	script.events = append([]bridgeSocketEvent(nil), expected.Events...)
	socket := &sdkBridgeSocket{owner: r, index: index, bridgeScriptSocket: script}
	r.sockets = append(r.sockets, socket)
	for _, operation := range []string{"connect", "wrap", "timeout"} {
		socket.mark(operation, 0)
	}
	r.connectionIndex++
	r.entropy.signing = false
	return socket, nil
}

type sdkBridgeSocket struct {
	*bridgeScriptSocket

	owner *sdkBridgeReplay
	index int
}

func (s *sdkBridgeSocket) Write(payload []byte) (int, error) {
	s.mark("send", s.cursor)
	original := len(payload)
	var err error
	if s.cursor == 0 {
		payload, err = s.upgrade(payload)
	} else {
		payload, err = bridgeFixtureUnmask(payload)
	}
	if err != nil {
		return 0, err
	}
	_, err = s.bridgeScriptSocket.Write(payload)
	return original, err
}

func (s *sdkBridgeSocket) Read(destination []byte) (int, error) {
	if len(s.pending) == 0 {
		s.mark("receive", s.cursor)
	}
	return s.bridgeScriptSocket.Read(destination)
}

func (s *sdkBridgeSocket) Close() error {
	if !s.closed {
		s.mark("close", s.cursor)
	}
	return s.bridgeScriptSocket.Close()
}

func (s *sdkBridgeSocket) mark(operation string, event int) {
	value := map[string]any{"surface": "socket", "connection": float64(s.index), "operation": operation}
	if operation == "send" || operation == "receive" || operation == "close" {
		value["event"] = float64(event)
	}
	s.owner.take(value)
}

func (s *sdkBridgeSocket) validateBootstrap(path string) {
	s.owner.t.Helper()
	raw, err := hex.DecodeString(path)
	if err != nil || hex.EncodeToString(raw) != path {
		s.owner.t.Fatal("noncanonical bootstrap path")
	}
	message := new(bridgepb.ClientMessage)
	if err = proto.Unmarshal(raw, message); err != nil {
		s.owner.t.Fatal(err)
	}
	canonical, err := proto.Marshal(message)
	if err != nil ||
		!bytes.Equal(canonical, raw) ||
		message.Connection == nil ||
		message.Subscription != nil ||
		message.Acknowledgement != nil ||
		len(message.ProtoReflect().GetUnknown()) != 0 {
		s.owner.t.Fatal("bootstrap envelope differs")
	}
	body := message.GetConnection()
	expected := s.owner.network.Connections[s.index].Bootstrap
	if base64.StdEncoding.EncodeToString(body.GetPublicKey()) != expected.Public ||
		base64.StdEncoding.EncodeToString(body.GetNonce()) != expected.Nonce ||
		body.GetExpiration().GetSeconds() != expected.Expiration ||
		len(body.ProtoReflect().GetUnknown()) != 0 ||
		len(body.GetExpiration().ProtoReflect().GetUnknown()) != 0 {
		s.owner.t.Fatal("bootstrap public key, nonce, or expiration differs")
	}
	public := body.GetPublicKey()
	nonce := body.GetNonce()
	signature := body.GetSignature()
	if len(public) != 65 || len(nonce) != 17 || nonce[0] != 0 || len(signature) < 2 || !bytes.Equal(signature[:2],
		[]byte{1, 3}) {
		s.owner.t.Fatal("bootstrap field layout differs")
	}
	// Validate the same uncompressed P-256 point before reconstructing its
	// coordinates for ECDSA. Exact Source public-key and nonce checks precede this.
	_, keyErr := ecdh.P256().NewPublicKey(public)
	x := new(big.Int).SetBytes(public[1:33])
	y := new(big.Int).SetBytes(public[33:])
	digest := sha256.Sum256(nonce)
	if keyErr != nil || !ecdsa.VerifyASN1(&ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, digest[:], signature[2:]) {
		s.owner.t.Fatal("bootstrap signature rejected")
	}
}

func TestNativeBridgeSDKReplay(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob("fixtures/synthetic/http/auth-bridge-*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 30 {
		t.Fatalf("bridge HTTP inventory changed: %d", len(paths))
	}
	for _, path := range paths {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".json"), func(t *testing.T) {
			t.Parallel()
			nativeBridgeSDKReplay(t, strings.TrimSuffix(filepath.Base(path), ".json"))
		})
	}
}

func nativeBridgeSDKReplay(t *testing.T, name string) {
	t.Helper()
	nativeBridgeSDKCase(t, name, false)
}

// Synthetic negative control extends the source-matched success transcript at
// the final trust boundary; it is not a captured provider or Python replay.
func TestNativeBridgeTrustFailureRetainsRotatedCredentials(t *testing.T) {
	t.Parallel()
	nativeBridgeSDKCase(t, "auth-bridge-modern-code-204", true)
}

func nativeBridgeSDKCase(t *testing.T, name string, lateFailure bool) {
	t.Helper()
	nativeBridgeSDKScenario(t, name, sdkBridgeControls{lateFailure: lateFailure,
		snapshot: false, overlap: false, after: nil})
}

// Synthetic unused phone metadata exposes RawMessage aliasing in State snapshots.
func TestNativeBridgeStateSnapshotsOwnPhoneUnion(t *testing.T) {
	t.Parallel()
	nativeBridgeSDKScenario(t, "auth-bridge-modern-prompt", sdkBridgeControls{lateFailure: false,
		snapshot: true, overlap: false, after: nil})
}

type sdkBridgeControls struct {
	lateFailure, snapshot, overlap bool
	after                          func(*icloud.NativeBridgeSession)
}

func nativeBridgeSDKScenario(t *testing.T, name string, controls sdkBridgeControls) {
	lateFailure, snapshot, control := controls.lateFailure, controls.snapshot, controls.after

	t.Helper()
	raw, transport, state := nativeFlowFixture(t, name)
	if lateFailure {
		transport = nativeBridgeTrustFailureFixture(t, raw)
	}
	nativeBridgeFixtureChallenge(t, raw, &state)
	if snapshot {
		var identifier icloud.TrustedPhoneNumberID
		authReplayDecode(t, json.RawMessage(`1`), &identifier)
		nonFTEU := true
		state.Challenge.PhoneNumbers = append(state.Challenge.PhoneNumbers,
			icloud.TrustedPhoneNumber{ID: identifier, Number: "synthetic-unused",
				PushMode: "sms", NonFTEU: &nonFTEU})
	}
	initial := authReplayObjectBytes(t, raw["initial_state"])
	originalState := state
	before := nativeBridgeMarshal(t, originalState)
	runner := new(sdkBridgeReplay)
	runner.t, runner.http, runner.overlap = t, transport, controls.overlap
	authReplayDecode(t, raw["bridge_network"], &runner.network)
	runner.entropy.replay = runner
	if entropy, ok := raw["entropy"]; ok {
		values := authReplayObjectBytes(t, entropy)
		var random []string
		authReplayDecode(t, values["random_bytes"], &random)
		if len(random) > 0 {
			value, _ := base64.StdEncoding.DecodeString(random[0])
			if len(value) == 256 {
				runner.entropy.initial = value
			}
		}
	}
	client, err := icloud.New(icloud.WithHTTPTransport(runner), icloud.WithRandomSource(&runner.entropy),
		icloud.WithClock(func() time.Time {
			return time.Unix(1700000000,
				0)
		}))
	if err != nil {
		t.Fatal(err)
	}
	var operation string
	authReplayDecode(t, raw["operation"], &operation)
	actions := []sdkBridgeAction{}
	if operation == "flow" {
		authReplayDecode(t, raw["inputs"], &actions)
	} else {
		actions = append(actions, sdkBridgeAction{Operation: operation, Inputs: nil})
	}
	flow, err := nativeBridgeActions(t, client, runner, state, initial, actions, name)
	session, result, state, values, responses := flow.session, flow.result, flow.state, flow.values, flow.responses
	_, expectedError := raw["error"]
	if (err != nil) != expectedError {
		t.Fatalf("expected error %v; got %v", expectedError, err)
	}
	if expectedError {
		nativeBridgeFailure(t, raw, operation, err, session, lateFailure)
	}
	if lateFailure {
		nativeFlowResponses(t, raw, result.Responses)
		nativeBridgeTrustFailure(t, result, session)
	} else if !expectedError {
		nativeBridgeSuccess(t, raw, name, operation, state, result, values, responses, session)
	}
	if control != nil {
		control(session)
	}
	if session != nil {
		if err := session.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := transport.AssertConsumed(); err != nil {
		t.Fatal(err)
	}
	if controls.overlap && !runner.overlapChecked {
		t.Fatal("overlap control did not reach step2")
	}
	if runner.timeline != len(runner.network.Timeline) {
		t.Fatalf("combined timeline consumed %d/%d", runner.timeline, len(runner.network.Timeline))
	}
	for _, socket := range runner.sockets {
		assertBridgeSocketConsumed(t, socket.bridgeScriptSocket)
	}
	if runner.entropy.nonceReads != len(runner.network.Connections) || len(runner.entropy.initial) != 0 {
		t.Fatal("declared nonce or SRP entropy not consumed")
	}
	if runner.entropy.private != len(runner.network.PrivateScalars) ||
		runner.entropy.prover != len(runner.network.ProverRandom) {
		t.Fatal("declared crypto entropy not consumed")
	}
	after := nativeBridgeMarshal(t, originalState)
	if !bytes.Equal(before, after) {
		t.Fatal("bridge changed caller-owned state")
	}
}

func nativeBridgeProjection(t *testing.T, raw map[string]json.RawMessage, session *icloud.NativeBridgeSession) {
	t.Helper()
	expected := authReplayObjectBytes(t, raw["result"])
	state := authReplayObjectBytes(t, expected["auth_state"])
	bridgeState, ok := state["bridge"]
	if !ok {
		return
	}
	if session == nil {
		t.Fatal("missing source bridge session")
	}
	fields := authReplayObjectBytes(t, bridgeState)
	var identifier, step string
	var active bool
	authReplayDecode(t, fields["session_uuid"], &identifier)
	authReplayDecode(t, fields["next_step"], &step)
	authReplayDecode(t, fields["websocket_active"], &active)
	progress, err := session.State()
	if err != nil {
		t.Fatal(err)
	}
	if progress.SessionID != identifier || progress.NextStep != step || progress.Active != active {
		t.Fatal("Source bridge progress differs")
	}
	var transaction *string
	authReplayDecode(t, fields["txnid"], &transaction)
	if transaction == nil {
		if !progress.TransactionID.IsNull() {
			t.Fatal("Source null transaction changed")
		}
	} else {
		value, err := progress.TransactionID.Get()
		if err != nil || value != *transaction || progress.Legacy != strings.HasSuffix(value, "_W") {
			t.Fatal("Source transaction or legacy selection differs")
		}
	}
	nativeBridgeSnapshotOwnership(t, session, progress)
}

func nativeBridgeSnapshotOwnership(t *testing.T, session *icloud.NativeBridgeSession,
	progress *icloud.NativeBridgeSessionState) {
	t.Helper()
	// Mutating detached snapshots must not change the owner.
	var identifierBefore []byte
	if len(progress.State.Challenge.PhoneNumbers) > 0 {
		identifier, err := progress.State.Challenge.PhoneNumbers[0].ID.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		identifierBefore = bytes.Clone(identifier)
		if len(identifier) > 0 {
			identifier[0] = '9'
		}
	}
	progress.State.Auth.Headers = append(progress.State.Auth.Headers,
		icloud.Header{Name: "synthetic-mutation", Value: "synthetic"})
	if len(progress.State.Challenge.PhoneNumbers) > 0 && progress.State.Challenge.PhoneNumbers[0].NonFTEU != nil {
		*progress.State.Challenge.PhoneNumbers[0].NonFTEU = !*progress.State.Challenge.PhoneNumbers[0].NonFTEU
	}
	after, err := session.State()
	if err != nil {
		t.Fatal(err)
	}
	if len(identifierBefore) > 0 {
		identifier, err := after.State.Challenge.PhoneNumbers[0].ID.MarshalJSON()
		if err != nil || !bytes.Equal(identifier, identifierBefore) {
			t.Fatal("State snapshot mutated session phone union bytes")
		}
	}
	for _, header := range after.State.Auth.Headers {
		if header.Name == "synthetic-mutation" {
			t.Fatal("State snapshot mutated session headers")
		}
	}
	if len(progress.State.Challenge.PhoneNumbers) > 0 &&
		progress.State.Challenge.PhoneNumbers[0].NonFTEU != nil &&
		*after.State.Challenge.PhoneNumbers[0].NonFTEU == *progress.State.Challenge.PhoneNumbers[0].NonFTEU {
		t.Fatal("State snapshot mutated session phone metadata")
	}
}

func nativeBridgeTrustFailureFixture(t *testing.T, raw map[string]json.RawMessage) *replay.HTTPTransport {
	t.Helper()
	var exchanges []replay.Exchange
	authReplayDecode(t, raw["exchanges"], &exchanges)
	exchanges = exchanges[:6]
	exchanges[5].Response = &replay.Response{Status: 503, Headers: []replay.Pair{{"Content-Type",
		"application/json"}, {"scnt", "synthetic-rotated-scnt"}, {"X-Apple-ID-Session-Id",
		"synthetic-rotated-session"}, {"X-Apple-Session-Token", "synthetic-rotated-token"},
		{"X-Apple-TwoSV-Trust-Token", "synthetic-rotated-trust"}, {"Set-Cookie",
			"synthetic-rotated-cookie=synthetic-rotated-value; Path=/; Secure; HttpOnly"}},
		Body: replay.Entity{Encoding: "base64", Value: json.RawMessage(`"e30="`)}}
	var network sdkBridgeNetwork
	authReplayDecode(t, raw["bridge_network"], &network)
	network.Timeline = network.Timeline[:len(network.Timeline)-1]
	raw["bridge_network"] = nativeBridgeMarshal(t, network)
	raw["exchanges"] = nativeBridgeMarshal(t, exchanges)
	delete(raw, "error")
	transport, fixtureErr := replay.NewHTTPTransport(exchanges)
	if fixtureErr != nil {
		t.Fatal(fixtureErr)
	}
	return transport
}

func nativeBridgeFixtureChallenge(t *testing.T, raw map[string]json.RawMessage, state *icloud.NativeAuthState) {
	t.Helper()
	nativeFixtureMFAState(t, raw, state)
	initial := authReplayObjectBytes(t, raw["initial_state"])
	var challenge map[string]json.RawMessage
	authReplayDecode(t, initial["auth_data"], &challenge)
	var account struct {
		Webservices map[string]map[string]string `json:"webservices"`
	}
	authReplayDecode(t, initial["account_data"], &account)
	state.Auth.DriveServiceURL = account.Webservices["drivews"]["url"]
	state.Auth.FindMyServiceURL = account.Webservices["findme"]["url"]
	state.Challenge.ProviderData = bytes.Clone(initial["auth_data"])
	if value, ok := challenge["authInitialRoute"]; ok {
		authReplayDecode(t, value, &state.Challenge.AuthInitialRoute)
	}
	if value, ok := challenge["hasTrustedDevices"]; ok {
		authReplayDecode(t, value, &state.Challenge.HasTrustedDevices)
	}
	state.Challenge.BridgeBootstrap = []byte(`{"twoSV":` + string(initial["auth_data"]) + `}`)
	if value, ok := challenge["authFactors"]; ok {
		authReplayDecode(t, value, &state.Challenge.AuthFactors)
	}
}

type sdkBridgeAction struct {
	Operation string   `json:"operation"`
	Inputs    []string `json:"inputs"`
}
type sdkBridgeFlow struct {
	session   *icloud.NativeBridgeSession
	result    *icloud.NativeAuthResult
	state     icloud.NativeAuthState
	values    []any
	responses []icloud.ResponseMetadata
}

func nativeBridgeActions(t *testing.T, client *icloud.SDK, runner *sdkBridgeReplay,
	state icloud.NativeAuthState, initial map[string]json.RawMessage,
	actions []sdkBridgeAction, name string) (*sdkBridgeFlow, error) {
	t.Helper()

	flow := new(sdkBridgeFlow)
	flow.state = state
	flow.values = []any{}
	flow.responses = []icloud.ResponseMetadata{}
	var err error
	for _, action := range actions {
		switch action.Operation {
		case "authenticate":
			var password string
			authReplayDecode(t, initial["synthetic_password"], &password)
			flow.result, err = client.Authenticate(t.Context(), icloud.AuthenticateRequest{Auth: flow.state.Auth,
				AccountName: flow.state.AccountName, Password: password, TrustToken: flow.state.TrustToken,
				AccountCountryCode: flow.state.AccountCountryCode, SavedState: &flow.state,
				AcceptTerms: false, ForceRefresh: false, PauseTwoFactor: false,
				Service: nil})
			if err == nil {
				flow.state = flow.result.State
				flow.responses = append(flow.responses, flow.result.Responses...)
				flow.session, err = client.OpenNativeBridgeSession(t.Context(),
					icloud.OpenNativeBridgeSessionRequest{Auth: flow.state.Auth,
						State: flow.state}, icloud.WithNativeBridgeDial(runner.dial))
			}
			flow.values = append(flow.values, nil)
		case "request_2fa_code":
			flow.result, err = client.RequestTwoFactorCode(t.Context(),
				icloud.RequestTwoFactorCodeRequest{Auth: flow.state.Auth, State: flow.state,
					PhoneNumberID: nil})
			if err == nil {
				flow.state = flow.result.State
				if flow.session == nil {
					flow.session, err = client.OpenNativeBridgeSession(t.Context(),
						icloud.OpenNativeBridgeSessionRequest{Auth: flow.state.Auth,
							State: flow.state}, icloud.WithNativeBridgeDial(runner.dial))
				}
			}
			if err == nil {
				progress, _ := flow.session.State()
				flow.state = progress.State
				flow.values = append(flow.values, true)
			}
		case "validate_2fa_code":
			flow.result, err = flow.session.VerifyCode(t.Context(), icloud.VerifyNativeBridgeCodeRequest{Code: action.Inputs[0]})
			if err == nil {
				flow.state = flow.result.State
				flow.values = append(flow.values, flow.result.Success)
			}
		default:
			t.Fatalf("unsupported bridge operation %s", action.Operation)
		}
		if err != nil {
			break
		}
		if flow.session != nil {
			progress, stateErr := flow.session.State()
			if stateErr != nil {
				t.Fatal(stateErr)
			}
			flow.state = progress.State
			runner.session = flow.session
		}
	}
	if err != nil && name == "auth-bridge-sms-fallback" {
		err = nativeBridgeSMSFallback(t, client, flow, err)
	}
	return flow, err
}

func nativeBridgeSMSFallback(t *testing.T, client *icloud.SDK, flow *sdkBridgeFlow, err error) error {
	t.Helper()

	if flow.session != nil {
		progress, stateErr := flow.session.State()
		if stateErr != nil {
			t.Fatal(stateErr)
		}
		flow.state = progress.State
	}
	var failure *icloud.ClientError
	if !errors.As(err, &failure) {
		t.Fatal(err)
	}
	flow.responses = append(flow.responses, failure.PriorResponses()...)
	if failure.StatusCode() != 0 {
		flow.responses = append(flow.responses, icloud.ResponseMetadata{StatusCode: failure.StatusCode(),
			Headers: failure.ResponseHeaders(), CookieScopeURL: failure.CookieScopeURL()})
	}
	selected := flow.state.Challenge.PhoneNumbers[0].ID
	flow.result, err = client.RequestTwoFactorCode(t.Context(),
		icloud.RequestTwoFactorCodeRequest{Auth: flow.state.Auth, State: flow.state,
			PhoneNumberID: &selected})
	if err == nil {
		flow.state = flow.result.State
		notice := protocol.AuthBridgeFallbackNoticeValue
		flow.state.DeliveryNotice = &notice
		flow.responses = append(flow.responses, flow.result.Responses...)
		flow.values = append(flow.values, true)
	}

	return err
}

func nativeBridgeFailure(t *testing.T, raw map[string]json.RawMessage,
	operation string, err error, _ *icloud.NativeBridgeSession,
	_ bool) {
	t.Helper()
	var failure *icloud.ClientError
	if !errors.As(err, &failure) {
		t.Fatalf("untyped bridge error %T", err)
	}
	metadata := failure.PriorResponses()
	if failure.StatusCode() != 0 {
		metadata = append(metadata, icloud.ResponseMetadata{StatusCode: failure.StatusCode(),
			Headers: failure.ResponseHeaders(), CookieScopeURL: failure.CookieScopeURL()})
	}
	nativeFlowResponses(t, raw, metadata)
	operationLabel := "OpenNativeBridgeSession"
	if operation == "flow" {
		operationLabel = "NativeBridgeSession.VerifyCode"
	}
	if !strings.Contains(failure.Error(), operationLabel) {
		t.Fatal("public bridge error operation label lost")
	}
	expectedFailure := authReplayObjectBytes(t, raw["error"])
	var message string
	if expected, ok := expectedFailure["message"]; ok {
		authReplayDecode(t, expected, &message)
	}
	if strings.Contains(message, "status 503") && failure.StatusCode() != http.StatusServiceUnavailable {
		t.Fatal("Source HTTP failure stage lost")
	}
	if strings.Contains(message, "decrypt") {
		var stage *bridge.ProtocolError
		if !errors.As(failure, &stage) || stage.Stage != "decrypt" {
			t.Fatal("Source decrypt failure cause lost")
		}
	}
	if strings.Contains(failure.Error(), "synthetic-") {
		t.Fatal("safe bridge error exposed synthetic provider content")
	}
}

func nativeBridgeSuccess(t *testing.T, raw map[string]json.RawMessage,
	name, operation string, state icloud.NativeAuthState, result *icloud.NativeAuthResult,
	values []any, responses []icloud.ResponseMetadata, session *icloud.NativeBridgeSession) {
	t.Helper()
	expected := authReplayObjectBytes(t, raw["result"])
	var want any
	authReplayDecode(t, expected["value"], &want)
	var actual any = values
	if operation != "flow" {
		actual = values[0]
	}
	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("operation values %v; expected %v", actual, want)
	}
	if result == nil {
		result = new(icloud.NativeAuthResult)
	}
	result.State = state
	if session != nil {
		progress, _ := session.State()
		result.Responses = nil
		result.Responses = append(result.Responses, responses...)
		result.Responses = append(result.Responses, progress.Responses...)
	}
	if session == nil || name == "auth-bridge-sms-fallback" {
		result.Responses = responses
	}
	nativeAssertState(t, raw, result)
	projection := icloud.ResumeSessionResult{Auth: result.State.Auth,
		TrustToken: result.State.TrustToken, AccountCountryCode: result.State.AccountCountryCode,
		AccountData: result.State.AccountData, Responses: result.Responses,
		TrustedSession: result.TrustedSession, RequiresTwoFactor: result.RequiresTwoFactor,
		RequiresTwoStep: result.RequiresTwoStep}
	assertResumedAuth(t, raw, &projection)
	nativeBridgeProjection(t, raw, session)
	expectedState := authReplayObjectBytes(t, expected["auth_state"])
	if notice, ok := expectedState["delivery_notice"]; ok {
		var expectedNotice string
		authReplayDecode(t, notice, &expectedNotice)
		if state.DeliveryNotice == nil || *state.DeliveryNotice != expectedNotice {
			t.Fatal("Source delivery notice changed")
		}
	}
	var provider any
	authReplayDecode(t, expectedState["challenge"], &provider)
	var actualProvider any
	if len(state.Challenge.ProviderData) == 0 {
		actualProvider = map[string]any{}
	} else {
		authReplayDecode(t, state.Challenge.ProviderData, &actualProvider)
	}
	if !reflect.DeepEqual(provider, actualProvider) {
		t.Fatalf("provider challenge changed: actual %v expected %v", actualProvider, provider)
	}
}

func nativeBridgeTrustFailure(t *testing.T, result *icloud.NativeAuthResult, session *icloud.NativeBridgeSession) {
	t.Helper()
	if result.Success || len(result.Responses) != 6 || result.Responses[5].StatusCode != http.StatusServiceUnavailable {
		t.Fatal("partial trust response metadata lost")
	}
	progress, stateErr := session.State()
	if stateErr != nil {
		t.Fatal(stateErr)
	}
	if progress.Active || progress.State.RequiresMFA || progress.State.CodeRequested {
		t.Fatal("partial trust progress flags differ")
	}
	if progress.State.Auth.SessionToken == nil ||
		*progress.State.Auth.SessionToken != "synthetic-rotated-token" ||
		progress.State.TrustToken != "synthetic-rotated-trust" {
		t.Fatal("partial trust token rotation lost")
	}
	headers := http.Header{}
	for _, header := range progress.State.Auth.Headers {
		headers.Add(header.Name, header.Value)
	}
	if headers.Get("Scnt") != "synthetic-rotated-scnt" ||
		headers.Get("X-Apple-ID-Session-Id") != "synthetic-rotated-session" {
		t.Fatal("partial trust continuation headers lost")
	}
	found := false
	for _, cookie := range progress.State.Auth.Cookies {
		if cookie.Name == "synthetic-rotated-cookie" && cookie.Value == "synthetic-rotated-value" {
			found = true
		}
	}
	if !found {
		t.Fatal("partial trust cookie rotation lost")
	}
}

func (r *sdkBridgeEntropy) readScalar(destination []byte) (int, error) {
	var scalar *big.Int
	switch {
	case r.signing:
		r.signatureReads++
		if r.signatureReads > 4 {
			return 0, errUnboundedSignatureEntropy
		}
		scalar = big.NewInt(1)
	case r.private == 0:
		if len(r.replay.network.PrivateScalars) != 1 {
			return 0, errUndeclaredBootstrapScalar
		}
		scalar, _ = new(big.Int).SetString(r.replay.network.PrivateScalars[r.private], 16)
		r.private++
	default:
		if r.prover >= len(r.replay.network.ProverRandom) {
			return 0, errUndeclaredProverScalar
		}
		sample := r.replay.network.ProverRandom[r.prover]
		r.prover++
		upper, ok := new(big.Int).SetString(sample.Upper, 0)
		if !ok || upper.Cmp(elliptic.P256().Params().N) != 0 {
			return 0, errIncorrectProverBound
		}
		scalar, _ = new(big.Int).SetString(sample.Value, 0)
	}
	if scalar == nil {
		return 0, errInvalidFixtureScalar
	}
	scalar.FillBytes(destination)
	return len(destination), nil
}

func (s *sdkBridgeSocket) upgrade(payload []byte) ([]byte, error) {
	lines := strings.Split(string(payload), "\r\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[0], "GET /v2/") || !strings.HasSuffix(lines[0], " HTTP/1.1") {
		return nil, errInvalidUpgradeTarget
	}
	path := strings.TrimSuffix(strings.TrimPrefix(lines[0], "GET /v2/"), " HTTP/1.1")
	s.validateBootstrap(path)
	lines[0] = "GET /v2/{signed_bootstrap} HTTP/1.1"
	key := base64.StdEncoding.EncodeToString([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})
	for index, line := range lines {
		if strings.HasPrefix(line, "Sec-WebSocket-Key:") {
			if line != "Sec-WebSocket-Key: "+key {
				return nil, errWebsocketEntropyDiffers
			}
			lines[index] = "Sec-WebSocket-Key: AAAAAAAAAAAAAAAAAAAAAA=="
		}
	}
	//nolint:gosec // RFC 6455 fixture accept digest, not a cryptographic signature.
	digest := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	s.events[1].Template = strings.ReplaceAll(s.events[1].Template,
		"{accept}", base64.StdEncoding.EncodeToString(digest[:]))
	payload = []byte(strings.Join(lines, "\r\n"))
	return payload, nil
}

func bridgeFixtureUnmask(payload []byte) ([]byte, error) {
	payload = append([]byte(nil), payload...)
	offset := 2
	switch payload[1] & 127 {
	case 126:
		offset += 2
	case 127:
		offset += 8
	}
	if len(payload) < offset+4 || !bytes.Equal(payload[offset:offset+4], []byte{0, 1, 2, 3}) {
		return nil, errFrameMaskEntropyDiffers
	}

	for index := offset + 4; index < len(payload); index++ {
		payload[index] ^= payload[offset+(index-offset-4)%4]
	}
	clear(payload[offset : offset+4])
	return payload, nil
}

// Synthetic caller cancellation occurs after the complete Source prompt boundary.
func TestNativeBridgeSDKCancellationAndIdempotentClose(t *testing.T) {
	t.Parallel()
	nativeBridgeSDKScenario(t, "auth-bridge-modern-prompt", sdkBridgeControls{lateFailure: false,
		snapshot: false, overlap: false, after: func(session *icloud.NativeBridgeSession) {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			_, err := session.VerifyCode(ctx, icloud.VerifyNativeBridgeCodeRequest{Code: "123456"})

			var failure *icloud.ClientError
			if !errors.Is(err, context.Canceled) || !errors.As(err, &failure) || failure.Kind() != icloud.Canceled {
				t.Fatalf("SDK cancellation ownership: %v", err)
			}

			nativeBridgeConcurrentClose(t, session)
			_, err = session.VerifyCode(t.Context(), icloud.VerifyNativeBridgeCodeRequest{Code: "123456"})
			if !errors.Is(err, bridge.ErrClosed) || !errors.As(err, &failure) || failure.Kind() != icloud.Closed {
				t.Fatalf("SDK closed owner: %v", err)
			}
			state, stateErr := session.State()
			if stateErr != nil {
				t.Fatal(stateErr)
			}

			if state.Active || state.State.CodeRequested {
				t.Fatal("cancellation did not preserve closed progress")
			}
		}})
}

func nativeBridgeConcurrentClose(t *testing.T, session *icloud.NativeBridgeSession) {
	t.Helper()
	var group sync.WaitGroup
	failures := make(chan error, 8)
	for range cap(failures) {
		group.Go(func() { failures <- session.Close() })
	}
	group.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
}

// Synthetic overlapping call runs while the original step2 HTTP boundary is in flight.
func TestNativeBridgeSDKOverlappingVerification(t *testing.T) {
	t.Parallel()
	nativeBridgeSDKScenario(t, "auth-bridge-modern-code-204", sdkBridgeControls{lateFailure: false,
		snapshot: false, overlap: true, after: nil})
}

// The failed-open owner retains the provider's metadata and cannot emit a second
// verification or close frame after its failed Source bootstrap boundary.
func TestNativeBridgeFailedOpenReturnsClosedOwner(t *testing.T) {
	t.Parallel()
	nativeBridgeSDKScenario(t, "auth-bridge-step0-refused", sdkBridgeControls{lateFailure: false,
		snapshot: false, overlap: false, after: func(session *icloud.NativeBridgeSession) {
			if session == nil {
				t.Fatal("failed open lost owned authentication progress")
			}
			state, err := session.State()
			if err != nil {
				t.Fatal(err)
			}

			if state.Active || state.SessionID != "" || state.NextStep != "" || !state.TransactionID.IsNull() {
				t.Fatal("failed-open owner exposed active challenge")
			}
			if len(state.Responses) != 1 || state.Responses[0].StatusCode != http.StatusServiceUnavailable {
				t.Fatal("failed-open owner lost current provider metadata")
			}
			_, err = session.VerifyCode(t.Context(), icloud.VerifyNativeBridgeCodeRequest{Code: "123456"})

			var failure *icloud.ClientError
			if !errors.Is(err, bridge.ErrClosed) || !errors.As(err, &failure) || failure.Kind() != icloud.Closed {
				t.Fatalf("failed-open verification: %v", err)
			}

			nativeBridgeConcurrentClose(t, session)
		}})
}

func nativeBridgeMarshal(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
