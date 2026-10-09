package bridge_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/portpowered/go-icloud/pkg/dependencies/bridge"
	models "github.com/portpowered/go-icloud/pkg/dependencymodels/bridge"
	pb "github.com/portpowered/go-icloud/pkg/dependencymodels/bridgepb"
	"google.golang.org/protobuf/proto"
	"io"
	"math/big"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	unitSalt   = "c3ludGhldGljIGJyaWRnZSBzYWx0"
	unitShareP = "04fc30efe89c6f7725e0f63829cb33f8e352cb4dffe4af21772f0b3350ac651344" +
		"03d925be2ccda6ed669d4e175e99748d93efc5452e180431d96045b041736d77"
	unitShareV = "0448495ec0ff0cbe6106180ccff6c7608ea4a84e7a423d83edd9e087dff9d4a7c3" +
		"668ab9426535c8ad435d17059c31392831956224d749b866f00e5ed1d9f0308a"
	unitConfirmationP = "099a63dd3cee72181f249c46771fd89db096763765c2b72b1092f637e2142e70"
	unitConfirmationV = "0156ff97b9b457bb434fbb490ec9e913b9c1e41cfa990d5f8b34dfaedb4cf0d7"
	unitCiphertext    = "AAABAgMEBQYHCAkKCzr1jVr/zVTK/4z70gGZWpOo8wzFN8I="
	unitSession       = "unit-flow"
)

type scalarEntropy struct{}

func (scalarEntropy) Read(payload []byte) (int, error) {
	clear(payload)

	if len(payload) > 0 {
		payload[len(payload)-1] = 42
	}

	return len(payload), nil
}

type zeroEntropy struct{}

func (zeroEntropy) Read(payload []byte) (int, error) {
	clear(payload)

	return len(payload), nil
}

type sessionSocket struct {
	mutex    sync.Mutex
	inbound  chan []byte
	outbound [][]byte
	closed   chan struct{}
	once     sync.Once
}

func newSessionSocket() *sessionSocket {
	socket := new(sessionSocket)
	socket.inbound = make(chan []byte, 8)
	socket.closed = make(chan struct{})

	return socket
}
func (socket *sessionSocket) ReadMessage(context.Context) ([]byte, error) {
	select {
	case <-socket.closed:
		return nil, io.ErrClosedPipe
	case payload := <-socket.inbound:
		return payload, nil
	}
}
func (socket *sessionSocket) SendBinary(_ context.Context, payload []byte) error {
	socket.mutex.Lock()
	defer socket.mutex.Unlock()

	socket.outbound = append(socket.outbound, bytes.Clone(payload))

	return nil
}
func (socket *sessionSocket) Close() error {
	socket.once.Do(func() { close(socket.closed) })

	return nil
}

func enqueueToken(t *testing.T, socket *sessionSocket, status pb.Status, timestamp uint64) {
	t.Helper()

	message := new(pb.ServerMessage)
	message.Connection = new(pb.ConnectionResponse)
	message.Connection.PushTokenBase64 = []byte("3q2+7w==")
	// Only generated nonnegative statuses are passed by these synthetic cases.
	//nolint:gosec // Test callers supply generated nonnegative status constants.
	message.Connection.Status = proto.Uint64(uint64(status))
	message.Connection.ServerTimestampSeconds = proto.Uint64(timestamp)
	enqueueMessage(t, socket, message)
}
func enqueuePush(t *testing.T, socket *sessionSocket, identifier string, step models.BridgeStep, extra string) {
	t.Helper()

	message := new(pb.ServerMessage)
	message.Push = new(pb.PushMessage)
	message.Push.Topic = []byte(models.DefaultTopic)
	message.Push.MessageId = proto.Uint64(1)
	message.Push.Payload = []byte(fmt.Sprintf(`{"flowid":%q,"nextStep":%d%s}`, identifier, step, extra))
	enqueueMessage(t, socket, message)
}
func enqueueMessage(t *testing.T, socket *sessionSocket, message *pb.ServerMessage) {
	t.Helper()

	payload, err := proto.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}

	socket.inbound <- payload
}
func unitOptions(t *testing.T, socket *sessionSocket) bridge.Options {
	t.Helper()

	options := new(bridge.Options)
	options.Entropy = scalarEntropy{}
	options.Clock = func() time.Time { return time.Unix(1700000000, 0) }
	options.Timeout = time.Second
	options.Open = func(_ context.Context, address string) (bridge.Socket, error) {
		verifyBootstrap(t, address)

		return socket, nil
	}
	options.Exchange = func(context.Context, models.BridgeExchange) error { return nil }
	options.Validate = func(context.Context, string, string) (bool, error) { return true, nil }

	return *options
}
func unitData() models.BridgeInitiateData {
	data := new(models.BridgeInitiateData)
	topic, env := string(models.DefaultTopic), string(models.ProductionEnvironment)
	data.ApnsTopic, data.ApnsEnvironment = &topic, &env

	return *data
}
func startUnitSession(t *testing.T, socket *sessionSocket, options bridge.Options) *bridge.Session {
	t.Helper()
	enqueueToken(t, socket, pb.Status_STATUS_OK, 0)
	enqueuePush(t, socket, unitSession, models.ProverShareStep, `,"salt":"`+unitSalt+`"`)

	session, err := bridge.Start(t.Context(), unitData(), options)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = session.Close() })

	return session
}
func decodeBootstrap(t *testing.T, address string) *pb.ConnectionRequest {
	t.Helper()

	u, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}

	payload, err := hex.DecodeString(strings.TrimPrefix(u.Path, "/v2/"))
	if err != nil {
		t.Fatal(err)
	}

	message := new(pb.ClientMessage)

	err = proto.Unmarshal(payload, message)
	if err != nil {
		t.Fatal(err)
	}

	return message.GetConnection()
}
func verifyBootstrap(t *testing.T, address string) {
	t.Helper()
	connection := decodeBootstrap(t, address)

	payload := connection.GetPublicKey()

	if len(payload) != 65 || len(connection.GetNonce()) != 17 || connection.GetExpiration().GetSeconds() != 86400 {
		t.Fatal("bootstrap layout mismatch")
	}

	key := new(ecdsa.PublicKey)
	key.Curve = elliptic.P256()
	key.X, key.Y = new(big.Int).SetBytes(payload[1:33]), new(big.Int).SetBytes(payload[33:])
	digest := sha256.Sum256(connection.GetNonce())

	signature := connection.GetSignature()
	if len(signature) < 2 || !bytes.Equal(signature[:2], []byte{1, 3}) {
		t.Fatal("signature prefix mismatch")
	}

	if !ecdsa.VerifyASN1(key, digest[:], signature[2:]) {
		t.Fatal("signature does not bind nonce")
	}
}
func base64Hex(t *testing.T, v string) string {
	t.Helper()

	payload, err := hex.DecodeString(v)
	if err != nil {
		t.Fatal(err)
	}

	return base64.StdEncoding.EncodeToString(payload)
}
func assertUnitProof(t *testing.T, firstSnapshot *string, expected string) {
	t.Helper()

	if firstSnapshot == nil || *firstSnapshot != base64Hex(t, expected) {
		t.Error("proof differs from pinned Python vector")
	}
}
func modernExchange(t *testing.T, steps *[]models.BridgeStep) bridge.Exchange {
	t.Helper()

	return func(_ context.Context, request models.BridgeExchange) error {
		*steps = append(*steps, request.NextStep)

		if request.Ptkn != "deadbeef" {
			t.Error("wrong token projection")
		}

		switch request.NextStep {
		case models.BootstrapStep:
		case models.ProverShareStep:
			assertUnitProof(t, request.Data, unitShareP)
		case models.ProverConfirmationStep:
			assertUnitProof(t, request.Data, unitConfirmationP)
		case models.CompletionStep:
			if request.Data == nil || *request.Data != string(models.DoneDataBase64) {
				t.Error("missing done marker")
			}
		}

		return nil
	}
}
func TestSessionNativeModernExchange(t *testing.T) {
	t.Parallel()

	socket := newSessionSocket()
	options := unitOptions(t, socket)
	steps := []models.BridgeStep{}
	options.Exchange = modernExchange(t, &steps)
	options.Validate = func(_ context.Context, identifier, code string) (bool, error) {
		if identifier != unitSession || code != "987654" {
			t.Error("wrong validation input")
		}

		return true, nil
	}
	session := startUnitSession(t, socket, options)
	serverProof := base64Hex(t, unitShareV) + string(models.ProofSeparator) + base64Hex(t, unitConfirmationV)
	proof := base64.StdEncoding.EncodeToString([]byte(serverProof))
	enqueuePush(t, socket, unitSession, models.ProverConfirmationStep, `,"data":"`+proof+`"`)
	enqueuePush(t, socket, unitSession, models.CompletionStep, `,"encryptedCode":"`+unitCiphertext+`"`)

	verified, err := session.VerifyCode(t.Context(), "654321")
	if err != nil || !verified {
		t.Fatalf("verification %t,%v", verified, err)
	}

	if fmt.Sprint(steps) != "[0 2 4 6]" {
		t.Fatal(steps)
	}

	if session.Active() {
		t.Fatal("verified session retained socket")
	}
}
func TestSessionCallerCancellationClosesIgnoringSocket(t *testing.T) {
	t.Parallel()

	socket := newSessionSocket()
	session := startUnitSession(t, socket, unitOptions(t, socket))

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	_, err := session.VerifyCode(ctx, "654321")
	if err == nil {
		t.Fatal("survived cancellation")
	}

	select {
	case <-socket.closed:
	default:
		t.Fatal("cancellation retained socket")
	}
}
func TestSessionRejectsWrongConfirmationAndSession(t *testing.T) {
	t.Parallel()

	for _, identifier := range []string{unitSession, "wrong-session"} {
		t.Run(identifier, func(t *testing.T) {
			t.Parallel()

			socket := newSessionSocket()
			session := startUnitSession(t, socket, unitOptions(t, socket))
			serverProof := base64Hex(t, unitShareV) + string(models.ProofSeparator) + base64Hex(t, unitConfirmationP)
			proof := base64.StdEncoding.EncodeToString([]byte(serverProof))
			enqueuePush(t, socket, identifier, models.ProverConfirmationStep, `,"data":"`+proof+`"`)

			verified, err := session.VerifyCode(t.Context(), "654321")
			if verified || identifier != unitSession && err == nil {
				t.Fatalf("verification %t,%v", verified, err)
			}
		})
	}
}
func TestSessionConcurrentVerificationAndOwnerCancellation(t *testing.T) {
	t.Parallel()

	owner, cancel := context.WithCancel(t.Context())
	defer cancel()

	socket := newSessionSocket()
	options := unitOptions(t, socket)
	entered := make(chan struct{})
	options.Exchange = func(_ context.Context, request models.BridgeExchange) error {
		if request.NextStep == models.ProverShareStep {
			close(entered)
		}

		return nil
	}

	enqueueToken(t, socket, pb.Status_STATUS_OK, 0)
	enqueuePush(t, socket, unitSession, models.ProverShareStep, `,"salt":"`+unitSalt+`"`)

	session, err := bridge.Start(owner, unitData(), options)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)

	go func() { _, failure := session.VerifyCode(t.Context(), "654321"); done <- failure }()

	<-entered

	_, err = session.VerifyCode(t.Context(), "654321")
	if !errors.Is(err, bridge.ErrBusy) {
		t.Fatal(err)
	}

	cancel()

	err = <-done
	if err == nil {
		t.Fatal("owner cancellation ignored")
	}

	err = session.Close()
	if err != nil {
		t.Fatal(err)
	}
}
func TestSessionRetriesOnlyProviderInvalidNonce(t *testing.T) {
	t.Parallel()

	first, second := newSessionSocket(), newSessionSocket()
	enqueueToken(t, first, pb.Status_STATUS_INVALID_NONCE, 123)
	enqueueToken(t, second, pb.Status_STATUS_OK, 0)
	enqueuePush(t, second, unitSession, models.ProverShareStep, `,"salt":"`+unitSalt+`"`)
	options := unitOptions(t, second)
	attempts := 0
	options.Open = func(_ context.Context, address string) (bridge.Socket, error) {
		verifyBootstrap(t, address)

		attempts++
		if attempts == 1 {
			return first, nil
		}

		if binary.BigEndian.Uint64(decodeBootstrap(t, address).GetNonce()[1:9]) != 123000 {
			t.Fatal("provider retry time ignored")
		}

		return second, nil
	}

	session, err := bridge.Start(t.Context(), unitData(), options)
	if err != nil {
		t.Fatal(err)
	}

	if attempts != 2 || !session.Active() {
		t.Fatal("retry did not establish session")
	}

	select {
	case <-first.closed:
	default:
		t.Fatal("failed socket retained")
	}

	err = session.Close()
	if err != nil {
		t.Fatal(err)
	}

	if session.Active() {
		t.Fatal("closed session active")
	}
}
func TestSessionCancelsRejectedZeroEntropy(t *testing.T) {
	t.Parallel()

	socket := newSessionSocket()
	options := unitOptions(t, socket)
	options.Entropy = zeroEntropy{}
	options.Open = func(context.Context, string) (bridge.Socket, error) {
		t.Fatal("zero scalar opened socket")

		return nil, io.ErrClosedPipe
	}

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	_, err := bridge.Start(ctx, unitData(), options)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}
func TestSessionSnapshotOwnsItsProjection(t *testing.T) {
	t.Parallel()

	socket := newSessionSocket()
	session := startUnitSession(t, socket, unitOptions(t, socket))

	firstSnapshot, err := session.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	*firstSnapshot.Payload.Flowid = "caller changed"

	secondSnapshot, err := session.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	if *secondSnapshot.Payload.Flowid != unitSession {
		t.Fatal("snapshot mutated session")
	}
}
