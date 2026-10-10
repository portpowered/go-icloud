package replay_test

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha1" //nolint:gosec // RFC 6455 requires SHA-1 solely for its upgrade accept digest (GO-15).
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/dependencies/bridgewebsocket"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/bridgepb"
	"github.com/portpowered/go-icloud/tests/replay"
	"google.golang.org/protobuf/proto"
)

var (
	errBridgeUndeclaredSRPEntropy              = errors.New("undeclared SRP entropy")
	errBridgeUnboundedOptionalSignatureEntropy = errors.New("unbounded optional signature entropy")
	errBridgeUnexpectedOptionalECDSAEntropy    = errors.New("unexpected optional ECDSA entropy")
	errBridgeInvalidFixtureNonce               = errors.New("invalid fixture nonce")
	errBridgeUndeclaredWebSocketOrUUIDEntropy  = errors.New("undeclared websocket or UUID entropy")
	errBridgeSecureDialDeadlineDiffers         = errors.New("secure dial deadline differs")
	errBridgeUndeclaredBridgeConnection        = errors.New("undeclared bridge connection")
	errBridgeUnexpectedSecureDialTarget        = errors.New("unexpected secure dial target")
	errBridgeUnboundedSignatureEntropy         = errors.New("unbounded signature entropy")
	errBridgeUndeclaredBootstrapScalar         = errors.New("undeclared bootstrap scalar")
	errBridgeUndeclaredProverScalar            = errors.New("undeclared prover scalar")
	errBridgeIncorrectProverBound              = errors.New("incorrect prover bound")
	errBridgeInvalidFixtureScalar              = errors.New("invalid fixture scalar")
	errBridgeInvalidUpgradeTarget              = errors.New("invalid upgrade target")
	errBridgeWebSocketEntropyDiffers           = errors.New("websocket entropy differs")
	errBridgeFrameMaskEntropyDiffers           = errors.New("frame mask entropy differs")
	errBridgeEntropyLength                     = errors.New("undeclared entropy length")
)

type sdkBridgeNetwork struct {
	//nolint:tagliatelle // The pinned Source fixture field is snake_case (LIB-12).
	PrivateScalars []string `json:"private_scalars"`
	ProverRandom   []struct {
		Upper string `json:"upper_hex"` //nolint:tagliatelle // The pinned Source fixture field is snake_case (LIB-12).
		Value string `json:"value_hex"` //nolint:tagliatelle // The pinned Source fixture field is snake_case (LIB-12).
	} `json:"prover_random"` //nolint:tagliatelle // The pinned Source fixture field is snake_case (LIB-12).
	Connections []struct {
		Connection bridgeSocketConnection `json:"connection"`
		Bootstrap  struct {
			Public string `json:"public_key"` //nolint:tagliatelle // The pinned Source fixture field is snake_case (LIB-12).
			Nonce  string `json:"nonce"`
			//nolint:tagliatelle // The pinned Source fixture field is snake_case (LIB-12).
			Expiration uint32 `json:"expiration_seconds"`
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
}

type sdkBridgeEntropy struct {
	mu                                               sync.Mutex
	replay                                           *sdkBridgeReplay
	initial                                          []byte
	private, prover                                  int
	signatureReads, nonceReads, wideReads, maskReads int
	signing, uuid                                    bool
}

func (r *sdkBridgeEntropy) Read(destination []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	clear(destination)

	switch len(destination) {
	case 256:
		if len(r.initial) != 256 {
			return 0, errBridgeUndeclaredSRPEntropy
		}

		copy(destination, r.initial)
		r.initial = nil
	case 1:
		r.signatureReads++
		if r.signatureReads > 4 {
			return 0, errBridgeUnboundedOptionalSignatureEntropy
		}

		if !r.signing {
			return 0, errBridgeUnexpectedOptionalECDSAEntropy
		}
	case 8:
		r.nonceReads++
		r.uuid = false
		r.signatureReads = 0
		r.replay.mu.Lock()
		index := r.replay.connectionIndex
		r.replay.mu.Unlock()

		if index >= len(r.replay.network.Connections) {
			return 0, io.EOF
		}

		nonce, err := base64.StdEncoding.DecodeString(r.replay.network.Connections[index].Bootstrap.Nonce)
		if err != nil || len(nonce) != 17 {
			return 0, errBridgeInvalidFixtureNonce
		}

		copy(destination, nonce[9:])

		r.signing = true
	case 16:
		r.wideReads++
		if r.wideReads > len(r.replay.network.Connections)+1 {
			return 0, errBridgeUndeclaredWebSocketOrUUIDEntropy
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
		return 0, fmt.Errorf("undeclared entropy length %d: %w", len(destination), errBridgeEntropyLength)
	}

	return len(destination), nil
}

func (r *sdkBridgeReplay) RoundTrip(request *http.Request) (*http.Response, error) {
	r.mu.Lock()
	index := r.httpIndex
	r.httpIndex++
	r.mu.Unlock()

	r.take(map[string]any{"surface": "http", "exchange": float64(index)})

	response, err := r.http.RoundTrip(request)
	if err != nil {
		return response, fmt.Errorf("replay bridge HTTP exchange: %w", err)
	}

	return response, nil
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
		return nil, errBridgeSecureDialDeadlineDiffers
	}

	r.mu.Lock()
	index := r.connectionIndex
	r.mu.Unlock()

	if index >= len(r.network.Connections) {
		return nil, errBridgeUndeclaredBridgeConnection
	}

	expected := r.network.Connections[index]

	if network != "tcp" || address != "bridge.example.invalid:443" {
		return nil, errBridgeUnexpectedSecureDialTarget
	}

	socket := &sdkBridgeSocket{owner: r, index: index, closeOnce: sync.Once{}, closeError: nil,
		bridgeScriptSocket: &bridgeScriptSocket{mu: sync.Mutex{},
			events: append([]bridgeSocketEvent(nil), expected.Events...), pending: nil,
			cursor: 0, closed: false, failure: nil}}

	r.mu.Lock()
	r.sockets = append(r.sockets, socket)
	r.mu.Unlock()

	for _, operation := range []string{"connect", "wrap", "timeout"} {
		socket.mark(operation, 0)
	}

	r.mu.Lock()
	r.connectionIndex++
	r.mu.Unlock()

	r.entropy.mu.Lock()
	r.entropy.signing = false
	r.entropy.mu.Unlock()

	return socket, nil
}

type sdkBridgeSocket struct {
	*bridgeScriptSocket

	owner      *sdkBridgeReplay
	index      int
	closeOnce  sync.Once
	closeError error
}

func (s *sdkBridgeSocket) Write(payload []byte) (int, error) {
	s.mu.Lock()
	cursor := s.cursor
	s.mu.Unlock()

	s.mark("send", cursor)

	original := len(payload)

	var err error
	if cursor == 0 {
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
	s.mu.Lock()
	empty, cursor := len(s.pending) == 0, s.cursor
	s.mu.Unlock()

	if empty {
		s.mark("receive", cursor)
	}

	return s.bridgeScriptSocket.Read(destination)
}

func (s *sdkBridgeSocket) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		cursor := s.cursor
		s.mu.Unlock()

		s.mark("close", cursor)
		s.closeError = s.bridgeScriptSocket.Close()
	})

	return s.closeError
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

	err = proto.Unmarshal(raw, message)
	if err != nil {
		s.owner.t.Fatal(err)
	}

	canonical, err := proto.Marshal(message)
	if err != nil || !bytes.Equal(canonical, raw) || message.GetConnection() == nil ||
		message.GetSubscription() != nil || message.GetAcknowledgement() != nil ||
		len(message.ProtoReflect().GetUnknown()) != 0 {
		s.owner.t.Fatal("bootstrap envelope differs")
	}

	body := message.GetConnection()

	expected := s.owner.network.Connections[s.index].Bootstrap

	if base64.StdEncoding.EncodeToString(body.GetPublicKey()) != expected.Public ||
		base64.StdEncoding.EncodeToString(body.GetNonce()) != expected.Nonce ||
		body.GetExpiration().GetSeconds() != expected.Expiration || len(body.ProtoReflect().GetUnknown()) != 0 ||
		len(body.GetExpiration().ProtoReflect().GetUnknown()) != 0 {
		s.owner.t.Fatal("bootstrap public key, nonce, or expiration differs")
	}

	public := body.GetPublicKey()
	nonce := body.GetNonce()

	signature := body.GetSignature()

	if len(public) != 65 || len(nonce) != 17 || nonce[0] != 0 || len(signature) < 2 ||
		!bytes.Equal(signature[:2], []byte{1, 3}) {
		s.owner.t.Fatal("bootstrap field layout differs")
	}

	validated, keyErr := ecdh.P256().NewPublicKey(public)
	if keyErr != nil {
		s.owner.t.Fatal(keyErr)
	}

	point := validated.Bytes()
	x := new(big.Int).SetBytes(point[1:33])
	y := new(big.Int).SetBytes(point[33:])

	digest := sha256.Sum256(nonce)

	if !ecdsa.VerifyASN1(&ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, digest[:], signature[2:]) {
		s.owner.t.Fatal("bootstrap signature rejected")
	}
}

func (r *sdkBridgeEntropy) readScalar(destination []byte) (int, error) {
	var scalar *big.Int

	switch {
	case r.signing:
		r.signatureReads++
		if r.signatureReads > 4 {
			return 0, errBridgeUnboundedSignatureEntropy
		}

		scalar = big.NewInt(1)
	case r.private == 0:
		if len(r.replay.network.PrivateScalars) != 1 {
			return 0, errBridgeUndeclaredBootstrapScalar
		}

		scalar, _ = new(big.Int).SetString(r.replay.network.PrivateScalars[r.private], 16)
		r.private++
	default:
		if r.prover >= len(r.replay.network.ProverRandom) {
			return 0, errBridgeUndeclaredProverScalar
		}

		sample := r.replay.network.ProverRandom[r.prover]
		r.prover++

		upper, ok := new(big.Int).SetString(sample.Upper, 0)
		if !ok || upper.Cmp(elliptic.P256().Params().N) != 0 {
			return 0, errBridgeIncorrectProverBound
		}

		scalar, _ = new(big.Int).SetString(sample.Value, 0)
	}

	if scalar == nil {
		return 0, errBridgeInvalidFixtureScalar
	}

	scalar.FillBytes(destination)

	return len(destination), nil
}

func (s *sdkBridgeSocket) upgrade(payload []byte) ([]byte, error) {
	lines := strings.Split(string(payload), "\r\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[0], "GET /v2/") || !strings.HasSuffix(lines[0], " HTTP/1.1") {
		return nil, errBridgeInvalidUpgradeTarget
	}

	path := strings.TrimSuffix(strings.TrimPrefix(lines[0], "GET /v2/"), " HTTP/1.1")
	s.validateBootstrap(path)

	lines[0] = "GET /v2/{signed_bootstrap} HTTP/1.1"
	key := base64.StdEncoding.EncodeToString([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})

	for index, line := range lines {
		if strings.HasPrefix(line, "Sec-WebSocket-Key:") {
			if line != "Sec-WebSocket-Key: "+key {
				return nil, errBridgeWebSocketEntropyDiffers
			}

			lines[index] = "Sec-WebSocket-Key: AAAAAAAAAAAAAAAAAAAAAA=="
		}
	}
	//nolint:gosec // RFC 6455 uses SHA-1 to bind the upgrade nonce, not for a security signature (GO-15).
	digest := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	s.events[1].Template = strings.ReplaceAll(s.events[1].Template, "{accept}",
		base64.StdEncoding.EncodeToString(digest[:]))
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
		return nil, errBridgeFrameMaskEntropyDiffers
	}

	for index := offset + 4; index < len(payload); index++ {
		payload[index] ^= payload[offset+(index-offset-4)%4]
	}

	clear(payload[offset : offset+4])

	return payload, nil
}

// Synthetic caller cancellation occurs after the complete Source prompt boundary.
type bridgeSocketConnection struct {
	URL       string `json:"url"`
	Origin    string `json:"origin"`
	UserAgent string `json:"user_agent"` //nolint:tagliatelle // The pinned Source fixture uses snake_case.
}
type bridgeSocketEvent struct {
	Direction string   `json:"direction"`
	Kind      string   `json:"kind"`
	Lines     []string `json:"lines"`
	Suffix    string   `json:"suffix"`
	Template  string   `json:"template"`
	Data      string   `json:"data"`
	Opcode    byte     `json:"opcode"`
	Payload   string   `json:"payload"`
	Error     string   `json:"error"`
}
type bridgeScriptSocket struct {
	mu      sync.Mutex
	events  []bridgeSocketEvent
	pending []byte
	cursor  int
	closed  bool
	failure error
}

func (s *bridgeScriptSocket) Write(payload []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, net.ErrClosed
	}

	if s.cursor >= len(s.events) {
		return 0, s.fail("unexpected socket send after transcript")
	}

	event := s.events[s.cursor]
	if event.Direction != "send" {
		return 0, s.fail("socket send out of order")
	}

	expected, err := expectedBridgeSend(event)
	if err != nil {
		return 0, err
	}

	if !bytes.Equal(payload, expected) {
		return 0, s.fail(fmt.Sprintf("socket send mismatch at event %d", s.cursor))
	}

	s.cursor++

	if event.Error != "" {
		return 0, fmt.Errorf("%s: %w", event.Error, io.ErrClosedPipe)
	}

	return len(payload), nil
}

func expectedBridgeSend(event bridgeSocketEvent) ([]byte, error) {
	if event.Kind == "upgrade" {
		lines := append([]string(nil), event.Lines...)
		lines = append(lines, "Sec-WebSocket-Key: AAAAAAAAAAAAAAAAAAAAAA==", "", "")

		return []byte(strings.Join(lines, "\r\n")), nil
	}

	payload, err := base64.StdEncoding.DecodeString(event.Payload)
	if err != nil {
		return nil, fmt.Errorf("fixture payload: %w", err)
	}

	header := []byte{0x80 | event.Opcode}
	length := len(payload)

	switch {
	case length < 126:
		header = append(header, 0x80|byte(length))
	case length < 65536:
		header = binary.BigEndian.AppendUint16(append(header, 0x80|126), uint16(length))
	default:
		header = binary.BigEndian.AppendUint64(append(header, 0x80|127), uint64(length))
	}

	header = append(header, 0, 0, 0, 0)

	return append(header, payload...), nil
}

func (s *bridgeScriptSocket) Read(destination []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, net.ErrClosed
	}

	if len(s.pending) == 0 {
		if s.cursor >= len(s.events) {
			return 0, s.fail("unexpected socket read after transcript")
		}

		event := s.events[s.cursor]
		if event.Direction != "receive" {
			return 0, s.fail("socket receive out of order")
		}

		s.cursor++

		var err error

		s.pending, err = expectedBridgeReceive(event)
		if err != nil {
			return 0, err
		}

		if len(s.pending) == 0 {
			return 0, io.EOF
		}
	}

	count := copy(destination, s.pending)
	s.pending = s.pending[count:]

	return count, nil
}

func expectedBridgeReceive(event bridgeSocketEvent) ([]byte, error) {
	if event.Kind == "upgrade" {
		//nolint:gosec // The pinned RFC 6455 fixture uses SHA-1 for its accept digest.
		digest := sha1.Sum([]byte("AAAAAAAAAAAAAAAAAAAAAA==258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		accept := base64.StdEncoding.EncodeToString(digest[:])

		suffix, err := base64.StdEncoding.DecodeString(event.Suffix)
		if err != nil {
			return nil, fmt.Errorf("fixture socket suffix: %w", err)
		}

		return append([]byte(strings.ReplaceAll(event.Template, "{accept}", accept)), suffix...), nil
	}

	payload, err := base64.StdEncoding.DecodeString(event.Data)
	if err != nil {
		return nil, fmt.Errorf("fixture socket bytes: %w", err)
	}

	return payload, nil
}

func (s *bridgeScriptSocket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	if s.cursor < len(s.events) && s.events[s.cursor].Direction == "close" {
		s.cursor++
	}

	return nil
}
func (*bridgeScriptSocket) LocalAddr() net.Addr              { return nil }
func (*bridgeScriptSocket) RemoteAddr() net.Addr             { return nil }
func (*bridgeScriptSocket) SetDeadline(time.Time) error      { return nil }
func (*bridgeScriptSocket) SetReadDeadline(time.Time) error  { return nil }
func (*bridgeScriptSocket) SetWriteDeadline(time.Time) error { return nil }

func (s *bridgeScriptSocket) fail(message string) error {
	s.failure = fmt.Errorf("%s: %w", message, bridgewebsocket.ErrProtocol)

	return s.failure
}
