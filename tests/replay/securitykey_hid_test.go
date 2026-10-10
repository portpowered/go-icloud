package replay_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/dependencies/securitykey"
)

const (
	hidReplayDevice                = "synthetic"
	hidReplayScalarBytes           = 32
	hidReplayPollDelay             = 250 * time.Millisecond
	hidReplayCommandOffset         = 5
	hidReplayPayloadOffset         = 7
	hidReplayCancelPadding         = 57
	hidReplayShortSignaturePadding = 50
)

type hidReplayStep struct {
	Writes []string `json:"writes"`
	Reads  []string `json:"reads"`
}

type hidReplayFixture struct {
	ClientData        string          `json:"clientData"`
	AuthenticatorData string          `json:"authenticatorData"`
	CredentialID      string          `json:"credentialId"`
	CredentialIDs     []string        `json:"credentialIds"`
	Signature         string          `json:"signature"`
	Steps             []hidReplayStep `json:"steps"`
	Error             string          `json:"error"`
}

type hidReplayEvent struct {
	write   bool
	packet  []byte
	failure error
}

type hidReplayConnection struct {
	t            *testing.T
	ctx          context.Context
	events       []hidReplayEvent
	closed       int
	closeFailure error
}

func (connection *hidReplayConnection) exchange(ctx context.Context, packet []byte, write bool) (int, error) {
	connection.t.Helper()
	if connection.closed != 0 || len(connection.events) == 0 {
		connection.t.Fatal("unexpected HID exchange or exchange after close")
	}

	event := connection.events[0]
	connection.events = connection.events[1:]
	if event.write != write {
		connection.t.Fatal("HID read/write ordering differs from Source transcript")
	}

	if ctx != connection.ctx {
		deadline, bounded := ctx.Deadline()
		if !write || len(event.packet) <= hidReplayCommandOffset || event.packet[hidReplayCommandOffset] != 0x91 ||
			!bounded || deadline.Before(time.Now()) || ctx.Err() != nil {
			connection.t.Fatal("HID exchange lost caller context or cancellation context is not bounded")
		}
	}

	if event.failure != nil {
		return 0, event.failure
	}

	if write {
		if !bytes.Equal(packet, event.packet) {
			connection.t.Fatalf("HID write differs: got %x, want %x", packet, event.packet)
		}

		return len(packet), nil
	}

	return copy(packet, event.packet), nil
}

func (connection *hidReplayConnection) Write(ctx context.Context, packet []byte) (int, error) {
	return connection.exchange(ctx, packet, true)
}

func (connection *hidReplayConnection) Read(ctx context.Context, packet []byte) (int, error) {
	return connection.exchange(ctx, packet, false)
}

func (connection *hidReplayConnection) Close() error {
	connection.closed++
	if connection.closed != 1 || len(connection.events) != 0 {
		connection.t.Fatal("HID handle closed twice or before consuming its ordered transcript")
	}

	return connection.closeFailure
}

type hidReplayBackend struct {
	t           *testing.T
	ctx         context.Context
	connections []*hidReplayConnection
	discovered  bool
	opened      int
}

func (backend *hidReplayBackend) Devices(ctx context.Context) ([]securitykey.Device, error) {
	if ctx != backend.ctx || backend.discovered || backend.opened != 0 {
		backend.t.Fatal("unexpected discovery or discovery context")
	}
	backend.discovered = true

	return []securitykey.Device{{ID: hidReplayDevice, Name: "Synthetic authenticator"}}, nil
}

//nolint:ireturn // Backend.Open is the public injectable HID boundary (GO-15).
func (backend *hidReplayBackend) Open(ctx context.Context, id string) (securitykey.Connection, error) {
	if ctx != backend.ctx || id != hidReplayDevice || backend.opened >= len(backend.connections) {
		backend.t.Fatal("unexpected HID open, device binding, or open context")
	}

	connection := backend.connections[backend.opened]
	backend.opened++

	return connection, nil
}

func hidReplayHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}

	return decoded
}

func hidReplayRead(t *testing.T, name string) hidReplayFixture {
	t.Helper()
	raw, err := fs.ReadFile(os.DirFS("fixtures/securitykey"), name)
	if err != nil {
		t.Fatal(err)
	}

	var fixture hidReplayFixture

	err = json.Unmarshal(raw, &fixture)
	if err != nil {
		t.Fatal(err)
	}

	return fixture
}

func hidReplayHandle(t *testing.T, steps []hidReplayStep) *hidReplayConnection {
	t.Helper()
	connection := &hidReplayConnection{t: t, ctx: t.Context(), events: nil, closed: 0, closeFailure: nil}
	for _, step := range steps {
		for _, packet := range step.Writes {
			connection.events = append(connection.events, hidReplayEvent{
				write: true, packet: hidReplayHex(t, packet), failure: nil})
		}
		for _, packet := range step.Reads {
			connection.events = append(connection.events, hidReplayEvent{
				write: false, packet: hidReplayHex(t, packet), failure: nil})
		}
	}

	return connection
}

func hidReplayProvider(t *testing.T, connections ...*hidReplayConnection) (*securitykey.Provider, *hidReplayBackend, *bytes.Reader, *int) {
	t.Helper()
	backend := &hidReplayBackend{t: t, ctx: t.Context(), connections: connections, discovered: false, opened: 0}
	nonce := hidReplayHex(t, "0001020304050607")
	entropy := append(bytes.Repeat(nonce, len(connections)), hidReplayHex(t,
		"0000000000000000000000000000000000000000000000000000000000000001")...)
	reader := bytes.NewReader(entropy)
	waits := 0
	provider, err := securitykey.New(backend, reader, securitykey.WithWaiter(func(ctx context.Context, delay time.Duration) error {
		if ctx != t.Context() || delay != hidReplayPollDelay {
			t.Fatal("unexpected presence retry context or delay")
		}

		waits++

		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}

	return provider, backend, reader, &waits
}

func hidReplayRequest(fixture hidReplayFixture) securitykey.Request {
	ids := fixture.CredentialIDs
	if ids == nil {
		ids = []string{"qrvM"}
	}

	return securitykey.Request{DeviceID: hidReplayDevice, RelyingPartyID: "apple.com",
		Origin: "https://apple.com", Challenge: "AQID", CredentialIDs: ids}
}

func TestSecurityKeySourceHIDReplay(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"python-ctap2-synthetic.json", "python-u2f-synthetic.json",
		"python-fallback-synthetic.json", "python-uv1-synthetic.json", "python-uv2-synthetic.json",
		"python-selection-synthetic.json", "python-zero-limit-synthetic.json", "python-uv-retry-synthetic.json",
		"python-uv-blocked-synthetic.json", "python-pin-required-synthetic.json"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fixture := hidReplayRead(t, name)
			discovery := hidReplayHandle(t, fixture.Steps[:1])
			assertion := hidReplayHandle(t, fixture.Steps)
			provider, backend, entropy, waits := hidReplayProvider(t, discovery, assertion)
			devices, err := provider.Devices(t.Context())
			if err != nil || len(devices) != 1 || devices[0].ID != hidReplayDevice ||
				devices[0].Name != "Synthetic authenticator" {
				t.Fatalf("discovery differs: %v, %v", devices, err)
			}

			result, err := provider.Assert(t.Context(), hidReplayRequest(fixture))
			if fixture.Error == "pinRequired" {
				if !errors.Is(err, securitykey.ErrPINRequired) {
					t.Fatalf("Source PIN failure differs: %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}

				hidReplayResult(t, result, fixture)
			}
			if backend.opened != len(backend.connections) || discovery.closed != 1 || assertion.closed != 1 {
				t.Fatal("discovery and assertion must own separate consumed handles")
			}
			expectedWaits := 0
			if name == "python-u2f-synthetic.json" {
				expectedWaits = 1
			}
			if *waits != expectedWaits {
				t.Fatal("Source presence retry count differs")
			}
			expectedEntropy := hidReplayScalarBytes
			if name == "python-uv1-synthetic.json" || name == "python-uv2-synthetic.json" {
				expectedEntropy = 0
			}
			if entropy.Len() != expectedEntropy {
				t.Fatal("unexpected nonce or PIN/UV entropy consumption")
			}
		})
	}
}

func hidReplayResult(t *testing.T, result securitykey.Assertion, fixture hidReplayFixture) {
	t.Helper()
	for name, pair := range map[string][2][]byte{
		"client data":        {result.ClientData, hidReplayHex(t, fixture.ClientData)},
		"authenticator data": {result.AuthenticatorData, hidReplayHex(t, fixture.AuthenticatorData)},
		"credential":         {result.CredentialID, hidReplayHex(t, fixture.CredentialID)},
		"signature":          {result.Signature, hidReplayHex(t, fixture.Signature)},
	} {
		if !bytes.Equal(pair[0], pair[1]) {
			t.Fatalf("%s differs from pinned Source", name)
		}
	}
	if len(result.UserHandle) != 0 {
		t.Fatal("unexpected user handle")
	}
}

// These adversarial synthetic variants retain the Source outbound transcript and
// change only the device response or owned handle failure named by the case.
func TestSecurityKeyHIDFailureReplay(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"short report", "wrong nonce", "wrong channel", "wrong command",
		"missing signature", "missing presence", "close failure", "cancel"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fixture := hidReplayRead(t, "python-u2f-synthetic.json")
			connection := hidReplayHandle(t, fixture.Steps)
			cause := securitykey.ErrProtocol
			switch name {
			case "short report":
				connection.events = connection.events[:2]
				connection.events[1].packet = connection.events[1].packet[:4]
			case "wrong nonce":
				connection.events = connection.events[:2]
				connection.events[1].packet[hidReplayPayloadOffset] = 0xff
			case "wrong channel":
				connection.events = connection.events[:2]
				connection.events[1].packet[0] = 0
			case "wrong command":
				connection.events = connection.events[:2]
				connection.events[1].packet[4] = 0x90
			case "missing signature":
				connection.events[len(connection.events)-1].packet = hidReplayHex(t,
					"0102030483000701000000009000"+strings.Repeat("00", hidReplayShortSignaturePadding))
			case "missing presence":
				connection.events[len(connection.events)-1].packet[hidReplayPayloadOffset] = 0
			case "close failure":
				cause = errors.New("synthetic handle close failure")
				connection.closeFailure = cause
			case "cancel":
				cause = context.Canceled
				connection.events = connection.events[:5]
				connection.events[4].failure = cause
				connection.events = append(connection.events, hidReplayEvent{write: true,
					packet: hidReplayHex(t, "0001020304910000"+strings.Repeat("00", hidReplayCancelPadding)), failure: nil})
			}

			provider, backend, _, _ := hidReplayProvider(t, connection)
			result, err := provider.Assert(t.Context(), hidReplayRequest(fixture))
			if !errors.Is(err, cause) || backend.opened != 1 || connection.closed != 1 {
				t.Fatalf("failure cause or handle ownership differs: %v", err)
			}
			if name == "close failure" {
				hidReplayResult(t, result, fixture)
			} else if len(result.Signature) != 0 {
				t.Fatal("failed ceremony returned a signed assertion")
			}
		})
	}
}
