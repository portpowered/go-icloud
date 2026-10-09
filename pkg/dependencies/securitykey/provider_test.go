package securitykey

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
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
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []Device{{ID: "synthetic", Name: "Synthetic authenticator"}}, nil
}
func (backend fakeBackend) Open(ctx context.Context, id string) (Connection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if id != "synthetic" {
		return nil, ErrProtocol
	}
	if backend.discovery != nil && backend.discovery.closed == 0 {
		return backend.discovery, nil
	}
	return backend.connection, nil
}

type fakeConnection struct {
	test              *testing.T
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

func (connection *fakeConnection) Close() error { connection.closed++; return connection.closeFailure }
func (connection *fakeConnection) Write(ctx context.Context, packet []byte) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if len(connection.writes) == 0 {
		connection.test.Fatal("unexpected HID write")
	}
	expected := connection.writes[0]
	connection.writes = connection.writes[1:]
	if !bytes.Equal(expected, packet) {
		connection.test.Fatalf("HID write differs: want %x, got %x", expected, packet)
	}
	return len(packet), nil
}
func (connection *fakeConnection) Read(ctx context.Context, packet []byte) (int, error) {
	if connection.readFailure != nil && connection.readCount >= connection.failureAfterReads {
		return 0, connection.readFailure
	}
	if connection.cancelOnRead || (connection.cancelAfterReads > 0 && connection.readCount >= connection.cancelAfterReads) {
		return 0, context.Canceled
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if len(connection.reads) == 0 {
		connection.test.Fatal("unexpected HID read")
	}
	next := connection.reads[0]
	connection.readCount++
	connection.reads = connection.reads[1:]
	return copy(packet, next), nil
}

func decodeHex(test *testing.T, value string) []byte {
	test.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		test.Fatal(err)
	}
	return decoded
}

func readTranscript(test *testing.T) transcript {
	return readTranscriptFile(test, "python-ctap2-synthetic.json")
}

func readTranscriptFile(test *testing.T, name string) transcript {
	test.Helper()
	raw, err := os.ReadFile("../../../tests/replay/fixtures/securitykey/" + name)
	if err != nil {
		test.Fatal(err)
	}
	var value transcript
	if err := json.Unmarshal(raw, &value); err != nil {
		test.Fatal(err)
	}
	return value
}

func transcriptConnection(test *testing.T, value transcript) *fakeConnection {
	test.Helper()
	connection := &fakeConnection{test: test}
	for _, step := range value.Steps {
		for _, packet := range step.Writes {
			connection.writes = append(connection.writes, decodeHex(test, packet))
		}
		for _, packet := range step.Reads {
			connection.reads = append(connection.reads, decodeHex(test, packet))
		}
	}
	return connection
}

func TestPythonCTAP2Transcript(test *testing.T) {
	test.Parallel()
	for _, name := range []string{"python-ctap2-synthetic.json", "python-u2f-synthetic.json", "python-uv1-synthetic.json", "python-uv2-synthetic.json", "python-selection-synthetic.json", "python-zero-limit-synthetic.json", "python-uv-retry-synthetic.json", "python-uv-blocked-synthetic.json", "python-pin-required-synthetic.json"} {
		test.Run(name, func(test *testing.T) { test.Parallel(); assertPythonTranscript(test, name) })
	}
}

func assertPythonTranscript(test *testing.T, name string) {
	test.Helper()
	fixture := readTranscriptFile(test, name)
	connection := transcriptConnection(test, fixture)
	discovery := transcriptConnection(test, transcript{Steps: fixture.Steps[:1]})
	entropy := append([]byte{0, 1, 2, 3, 4, 5, 6, 7}, make([]byte, 31)...)
	entropy = append(entropy, 1)
	entropy = append([]byte{0, 1, 2, 3, 4, 5, 6, 7}, entropy...)
	waits := 0
	provider, err := New(fakeBackend{connection: connection, discovery: discovery}, bytes.NewReader(entropy), WithWaiter(func(ctx context.Context, delay time.Duration) error {
		waits++
		if delay != presencePollDelay {
			test.Fatalf("unexpected delay %v", delay)
		}
		return ctx.Err()
	}))
	if err != nil {
		test.Fatal(err)
	}
	devices, err := provider.Devices(test.Context())
	if err != nil || len(devices) != 1 {
		test.Fatalf("devices: %v, %v", devices, err)
	}
	ids := fixture.CredentialIDs
	if ids == nil {
		ids = []string{"qrvM"}
	}
	result, err := provider.Assert(test.Context(), Request{DeviceID: devices[0].ID, RelyingPartyID: "apple.com", Origin: "https://apple.com", Challenge: "AQID", CredentialIDs: ids})
	if fixture.Error == "pinRequired" {
		if !errors.Is(err, ErrPINRequired) || connection.closed != 1 || len(connection.writes) != 0 || len(connection.reads) != 0 {
			test.Fatalf("PIN failure differs from Source: %v", err)
		}
		return
	}
	if err != nil {
		test.Fatal(err)
	}
	if !bytes.Equal(result.ClientData, decodeHex(test, fixture.ClientData)) || !bytes.Equal(result.AuthenticatorData, decodeHex(test, fixture.AuthenticatorData)) || !bytes.Equal(result.CredentialID, decodeHex(test, fixture.CredentialID)) || !bytes.Equal(result.Signature, decodeHex(test, fixture.Signature)) {
		test.Fatal("assertion differs from Python reference")
	}
	if connection.closed != 1 || len(connection.writes) != 0 || len(connection.reads) != 0 {
		test.Fatal("ceremony did not consume transcript or release handle")
	}
	if discovery.closed != 1 || len(discovery.writes) != 0 || len(discovery.reads) != 0 {
		test.Fatal("discovery did not initialize and release its detached handle")
	}
	if name == "python-u2f-synthetic.json" && waits != 1 {
		test.Fatal("U2F presence was not polled")
	}
}

func TestCancellationSendsCancelAndCloses(test *testing.T) {
	test.Parallel()
	fixture := readTranscript(test)
	connection := transcriptConnection(test, fixture)
	// Cancel immediately after allocation; cancellation uses the allocated channel.
	connection.reads = connection.reads[:1]
	connection.writes = connection.writes[:2]
	cancelPacket := make([]byte, int(wire.ReportBytes)+1)
	copy(cancelPacket[1:], []byte{1, 2, 3, 4})
	cancelPacket[5] = byte(wire.CancelCommand) | byte(wire.InitialFlag)
	connection.writes = append(connection.writes, cancelPacket)
	connection.cancelAfterReads = 1
	provider, err := New(fakeBackend{connection: connection}, bytes.NewReader([]byte{0, 1, 2, 3, 4, 5, 6, 7}))
	if err != nil {
		test.Fatal(err)
	}
	_, err = provider.Assert(test.Context(), Request{DeviceID: "synthetic", RelyingPartyID: "apple.com", Origin: "https://apple.com", Challenge: "AQID"})
	if !errors.Is(err, context.Canceled) {
		test.Fatalf("cancellation cause missing: %v", err)
	}
	if len(connection.writes) != 0 {
		test.Fatal("cancel frame was not sent")
	}
	if connection.closed != 1 {
		test.Fatal("canceled ceremony did not close handle exactly once")
	}
}

func TestRejectInvalidFrames(test *testing.T) {
	test.Parallel()
	cases := map[string][]byte{"short": make([]byte, 4), "orphan continuation": make([]byte, int(wire.ReportBytes)), "wrong command": make([]byte, int(wire.ReportBytes))}
	cases["wrong command"][4] = byte(wire.InitCommand) | byte(wire.InitialFlag)
	for name, packet := range cases {
		test.Run(name, func(test *testing.T) {
			test.Parallel()
			connection := &fakeConnection{test: test, reads: [][]byte{packet}}
			channel := &channel{connection: connection}
			_, err := channel.read(test.Context(), byte(wire.CBORCommand))
			if !errors.Is(err, ErrProtocol) {
				test.Fatalf("invalid frame accepted: %v", err)
			}
		})
	}
}

func TestRejectInvalidAssertionBindings(test *testing.T) {
	test.Parallel()
	fixture := readTranscript(test)
	credential := &wire.Credential{Id: decodeHex(test, fixture.CredentialID), Type: wire.PublicKey}
	valid := wire.AssertionResponse{AuthData: decodeHex(test, fixture.AuthenticatorData), Signature: decodeHex(test, fixture.Signature), Credential: credential}
	cases := map[string]wire.AssertionResponse{"wrong relying party": valid, "missing presence": valid, "missing signature": valid, "different credential": valid}
	missingPresence := valid
	missingPresence.AuthData = append([]byte(nil), valid.AuthData...)
	missingPresence.AuthData[32] = 0
	cases["missing presence"] = missingPresence
	missingSignature := valid
	missingSignature.Signature = nil
	cases["missing signature"] = missingSignature
	different := valid
	different.Credential = &wire.Credential{Id: []byte{1}, Type: wire.PublicKey}
	cases["different credential"] = different
	for name, response := range cases {
		test.Run(name, func(test *testing.T) {
			test.Parallel()
			rp := "apple.com"
			if name == "wrong relying party" {
				rp = "example.com"
			}
			_, err := projectAssertion(rp, nil, credential, &response)
			if !errors.Is(err, ErrProtocol) {
				test.Fatalf("invalid binding accepted: %v", err)
			}
		})
	}
}
