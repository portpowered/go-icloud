//nolint:testpackage // White-box protocol and injected HID lifecycle tests require unexported seams (GO-15).
package securitykey

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"testing"
	"time"

	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
)

const (
	syntheticDeviceID       = "synthetic"
	syntheticChallenge      = "AQID"
	syntheticCredentialID   = "qrvM"
	syntheticRelyingPartyID = "apple.com"
	syntheticOrigin         = "https://apple.com"
	syntheticU2FFixture     = "python-u2f-synthetic.json"
	wrongRelyingPartyCase   = "wrong relying party"
)

var (
	errSyntheticRead        = errors.New("synthetic read failure")
	errSyntheticClose       = errors.New("synthetic close failure")
	errSyntheticEnumeration = errors.New("synthetic enumeration failure")
)

type transcriptStep struct {
	Writes []string `json:"writes"`
	Reads  []string `json:"reads"`
}
type transcript struct {
	ClientData        string           `json:"clientData"`
	AuthenticatorData string           `json:"authenticatorData"`
	CredentialID      string           `json:"credentialId"`
	CredentialIDs     []string         `json:"credentialIds"`
	Signature         string           `json:"signature"`
	Steps             []transcriptStep `json:"steps"`
	Error             string           `json:"error"`
}

type fakeBackend struct {
	connection *fakeConnection
	discovery  *fakeConnection
}

func (backend fakeBackend) Devices(ctx context.Context) ([]Device, error) {
	err := ctx.Err()
	if err != nil {
		return nil, fmt.Errorf("synthetic discovery context: %w", err)
	}

	return []Device{{ID: syntheticDeviceID, Name: "Synthetic authenticator"}}, nil
}

//nolint:ireturn // The fake implements the injected Backend interface returning Connection.
func (backend fakeBackend) Open(ctx context.Context, deviceID string) (Connection, error) {
	err := ctx.Err()
	if err != nil {
		return nil, fmt.Errorf("synthetic open context: %w", err)
	}

	if deviceID != syntheticDeviceID {
		return nil, ErrProtocol
	}

	if backend.discovery != nil && backend.discovery.closed == 0 {
		return backend.discovery, nil
	}

	return backend.connection, nil
}

type fakeConnection struct {
	t                 *testing.T
	writes            [][]byte
	reads             [][]byte
	closed            int
	cancelOnRead      bool
	cancelAfterReads  int
	readCount         int
	readFailure       error
	failureAfterReads int
	closeFailure      error
}

func (connection *fakeConnection) Close() error {
	connection.closed++

	return connection.closeFailure
}
func (connection *fakeConnection) Write(ctx context.Context, packet []byte) (int, error) {
	err := ctx.Err()
	if err != nil {
		return 0, fmt.Errorf("synthetic write context: %w", err)
	}

	if len(connection.writes) == 0 {
		connection.t.Fatal("unexpected HID write")
	}

	expected := connection.writes[0]

	connection.writes = connection.writes[1:]

	if !bytes.Equal(expected, packet) {
		connection.t.Fatalf("HID write differs: want %x, got %x", expected, packet)
	}

	return len(packet), nil
}
func (connection *fakeConnection) Read(ctx context.Context, packet []byte) (int, error) {
	if connection.readFailure != nil && connection.readCount >= connection.failureAfterReads {
		return 0, connection.readFailure
	}

	if connection.cancelOnRead ||
		(connection.cancelAfterReads > 0 && connection.readCount >= connection.cancelAfterReads) {
		return 0, context.Canceled
	}

	err := ctx.Err()
	if err != nil {
		return 0, fmt.Errorf("synthetic read context: %w", err)
	}

	if len(connection.reads) == 0 {
		connection.t.Fatal("unexpected HID read")
	}

	next := connection.reads[0]
	connection.readCount++
	connection.reads = connection.reads[1:]

	return copy(packet, next), nil
}

func decodeHex(t *testing.T, value string) []byte {
	t.Helper()

	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}

	return decoded
}

func readTranscript(t *testing.T) transcript {
	t.Helper()

	return readTranscriptFile(t, "python-ctap2-synthetic.json")
}

func readTranscriptFile(t *testing.T, name string) transcript {
	t.Helper()

	raw, err := fs.ReadFile(os.DirFS("../../../tests/replay/fixtures/securitykey"), name)
	if err != nil {
		t.Fatal(err)
	}

	var value transcript

	err = json.Unmarshal(raw, &value)
	if err != nil {
		t.Fatal(err)
	}

	return value
}

func transcriptConnection(t *testing.T, value transcript) *fakeConnection {
	t.Helper()
	connection := &fakeConnection{
		t:                 t,
		writes:            nil,
		reads:             nil,
		closed:            0,
		cancelOnRead:      false,
		cancelAfterReads:  0,
		readCount:         0,
		readFailure:       nil,
		failureAfterReads: 0,
		closeFailure:      nil,
	}

	for _, step := range value.Steps {
		for _, packet := range step.Writes {
			connection.writes = append(connection.writes, decodeHex(t, packet))
		}

		for _, packet := range step.Reads {
			connection.reads = append(connection.reads, decodeHex(t, packet))
		}
	}

	return connection
}

func TestPythonCTAP2Transcript(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"python-ctap2-synthetic.json",
		syntheticU2FFixture,
		"python-fallback-synthetic.json",
		"python-uv1-synthetic.json",
		"python-uv2-synthetic.json",
		"python-selection-synthetic.json",
		"python-zero-limit-synthetic.json",
		"python-uv-retry-synthetic.json",
		"python-uv-blocked-synthetic.json",
		"python-pin-required-synthetic.json",
	} {
		t.Run(name, func(t *testing.T) { t.Parallel(); assertPythonTranscript(t, name) })
	}
}

func assertPythonTranscript(t *testing.T, name string) {
	t.Helper()
	fixture := readTranscriptFile(t, name)
	connection := transcriptConnection(t, fixture)
	discoveryFixture := fixture
	discoveryFixture.Steps = fixture.Steps[:1]
	discovery := transcriptConnection(t, discoveryFixture)
	provider, waits := transcriptProvider(t, connection, discovery)

	devices, err := provider.Devices(t.Context())
	if err != nil || len(devices) != 1 {
		t.Fatalf("devices: %v, %v", devices, err)
	}

	ids := fixture.CredentialIDs
	if ids == nil {
		ids = []string{syntheticCredentialID}
	}

	result, err := provider.Assert(t.Context(), Request{
		DeviceID:       devices[0].ID,
		RelyingPartyID: syntheticRelyingPartyID,
		Origin:         syntheticOrigin,
		Challenge:      syntheticChallenge,
		CredentialIDs:  ids,
	})
	if fixture.Error == "pinRequired" {
		if !errors.Is(err, ErrPINRequired) {
			t.Fatalf("PIN failure differs from Source: %v", err)
		}

		assertConsumed(t, connection)

		return
	}

	if err != nil {
		t.Fatal(err)
	}

	assertTranscriptResult(t, result, fixture)
	assertConsumed(t, connection)
	assertConsumed(t, discovery)

	if name == syntheticU2FFixture && *waits != 1 {
		t.Fatal("U2F presence was not polled")
	}
}

func transcriptProvider(t *testing.T, connection, discovery *fakeConnection) (*Provider, *int) {
	t.Helper()

	nonce := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	entropy := append(append([]byte(nil), nonce...), make([]byte, 31)...)
	entropy = append(entropy, 1)
	entropy = append(append([]byte(nil), nonce...), entropy...)
	waits := 0
	waiter := func(ctx context.Context, delay time.Duration) error {
		waits++

		if delay != presencePollDelay {
			t.Fatalf("unexpected delay %v", delay)
		}

		err := ctx.Err()
		if err != nil {
			return fmt.Errorf("synthetic waiter context: %w", err)
		}

		return nil
	}

	provider, err := New(
		fakeBackend{connection: connection, discovery: discovery},
		bytes.NewReader(entropy),
		WithWaiter(waiter),
	)
	if err != nil {
		t.Fatal(err)
	}

	return provider, &waits
}

func assertConsumed(t *testing.T, connection *fakeConnection) {
	t.Helper()

	if connection.closed != 1 || len(connection.writes) != 0 || len(connection.reads) != 0 {
		t.Fatal("transcript was not consumed or handle was not closed exactly once")
	}
}

type transcriptBytesExpectation struct {
	name     string
	actual   []byte
	expected string
}

func assertTranscriptResult(t *testing.T, result Assertion, fixture transcript) {
	t.Helper()

	comparisons := []transcriptBytesExpectation{
		{name: "client data", actual: result.ClientData, expected: fixture.ClientData},
		{name: "authenticator data", actual: result.AuthenticatorData, expected: fixture.AuthenticatorData},
		{name: "credential", actual: result.CredentialID, expected: fixture.CredentialID},
		{name: "signature", actual: result.Signature, expected: fixture.Signature},
	}
	for _, comparison := range comparisons {
		if !bytes.Equal(comparison.actual, decodeHex(t, comparison.expected)) {
			t.Fatalf("%s differs from Python reference", comparison.name)
		}
	}
}

func TestCancellationSendsCancelAndCloses(t *testing.T) {
	t.Parallel()
	fixture := readTranscript(t)
	connection := transcriptConnection(t, fixture)
	// Cancel immediately after allocation; cancellation uses the allocated channel.
	connection.reads = connection.reads[:1]
	connection.writes = connection.writes[:2]
	cancelPacket := make([]byte, int(wire.ReportBytes)+1)
	copy(cancelPacket[1:], []byte{
		1,
		2,
		3,
		4,
	})

	cancelPacket[5] = byte(wire.CancelCommand) | byte(wire.InitialFlag)
	connection.writes = append(connection.writes, cancelPacket)
	connection.cancelAfterReads = 1

	provider, err := New(fakeBackend{
		connection: connection,
		discovery:  nil,
	}, bytes.NewReader([]byte{
		0,
		1,
		2,
		3,
		4,
		5,
		6,
		7,
	}))
	if err != nil {
		t.Fatal(err)
	}

	_, err = provider.Assert(t.Context(), Request{
		DeviceID:       syntheticDeviceID,
		RelyingPartyID: syntheticRelyingPartyID,
		Origin:         syntheticOrigin,
		Challenge:      syntheticChallenge,
		CredentialIDs:  nil,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation cause missing: %v", err)
	}

	if len(connection.writes) != 0 {
		t.Fatal("cancel frame was not sent")
	}

	if connection.closed != 1 {
		t.Fatal("canceled ceremony did not close handle exactly once")
	}
}

func TestRejectInvalidFrames(t *testing.T) {
	t.Parallel()

	cases := map[string][]byte{
		"short":               make([]byte, 4),
		"orphan continuation": make([]byte, int(wire.ReportBytes)),
		"wrong command":       make([]byte, int(wire.ReportBytes)),
	}

	cases["wrong command"][4] = byte(wire.InitCommand) | byte(wire.InitialFlag)

	for name, packet := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			connection := &fakeConnection{
				t: t,
				reads: [][]byte{
					packet,
				},
				writes:            nil,
				closed:            0,
				cancelOnRead:      false,
				cancelAfterReads:  0,
				readCount:         0,
				readFailure:       nil,
				failureAfterReads: 0,
				closeFailure:      nil,
			}
			channel := &channel{
				connection:      connection,
				id:              0,
				ctap2:           false,
				wait:            nil,
				entropy:         nil,
				maxMessageBytes: 0,
			}

			_, err := channel.read(t.Context(), byte(wire.CBORCommand))
			if !errors.Is(err, ErrProtocol) {
				t.Fatalf("invalid frame accepted: %v", err)
			}
		})
	}
}

func TestRejectInvalidAssertionBindings(t *testing.T) {
	t.Parallel()
	fixture := readTranscript(t)
	credential := &wire.Credential{Id: decodeHex(t, fixture.CredentialID), Type: wire.PublicKey}
	valid := wire.AssertionResponse{
		AuthData:            decodeHex(t, fixture.AuthenticatorData),
		Signature:           decodeHex(t, fixture.Signature),
		Credential:          credential,
		NumberOfCredentials: nil,
		User:                nil,
	}
	cases := map[string]wire.AssertionResponse{
		wrongRelyingPartyCase:  valid,
		"missing presence":     valid,
		"missing signature":    valid,
		"different credential": valid,
	}
	missingPresence := valid
	missingPresence.AuthData = append([]byte(nil), valid.AuthData...)
	missingPresence.AuthData[32] = 0
	cases["missing presence"] = missingPresence
	missingSignature := valid
	missingSignature.Signature = nil
	cases["missing signature"] = missingSignature
	different := valid
	different.Credential = &wire.Credential{Id: []byte{
		1,
	}, Type: wire.PublicKey}

	cases["different credential"] = different
	for name, response := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rp := syntheticRelyingPartyID
			if name == wrongRelyingPartyCase {
				rp = "example.com"
			}

			_, err := projectAssertion(rp, nil, credential, &response)
			if !errors.Is(err, ErrProtocol) {
				t.Fatalf("invalid binding accepted: %v", err)
			}
		})
	}
}
