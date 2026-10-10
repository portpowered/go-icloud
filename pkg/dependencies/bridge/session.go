package bridge

import (
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	models "github.com/portpowered/go-icloud/pkg/dependencymodels/bridge"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/bridgepb"
)

const (
	bootstrapAttempts = 2
	bootstrapStage    = "bootstrap"
)

var (
	// ErrClosed identifies a detached bridge connection.
	ErrClosed = errors.New("bridge session is closed")
	// ErrBusy identifies an overlapping verification attempt.
	ErrBusy    = errors.New("bridge verification is already running")
	errSession = errors.New("bridge session does not match expected challenge")
)

// OpenSocket owns a connection created for the specified schema-bound URL.
type OpenSocket func(context.Context, string) (Socket, error)

// Exchange posts a generated bridge step using session-owned HTTP credentials.
type Exchange func(context.Context, models.BridgeExchange) error

// ValidateCode posts the decrypted final code and returns its acknowledged status.
type ValidateCode func(context.Context, string, string) (bool, error)

// Options configures one account-bound bootstrap and its explicit dependencies.
type Options struct {
	Entropy  io.Reader
	Clock    func() time.Time
	Timeout  time.Duration
	Open     OpenSocket
	Exchange Exchange
	Validate ValidateCode
}

// Session owns one ephemeral trusted-device connection and its challenge.
// Close interrupts active work; verification never retries an uncertain HTTP step.
type Session struct {
	mutex      sync.Mutex
	closeOnce  sync.Once
	closeError error
	socket     Socket
	push       *Push
	identifier string
	token      string
	topic      string
	options    Options
	ctx        context.Context //nolint:containedctx // Explicit connection owner binds cancellation to its lifecycle.
	cancel     context.CancelFunc
	busy       atomic.Bool
}

// Start obtains the push token, subscribes, sends step zero, and then reads the prompt.
func Start(ctx context.Context, data models.BridgeInitiateData, options Options) (*Session, error) {
	err := ctx.Err()
	if err != nil {
		return nil, &ProtocolError{Stage: bootstrapStage, Cause: err}
	}

	if !validOptions(options) {
		return nil, &ProtocolError{Stage: "configuration", Cause: errBootstrap}
	}

	host, err := SocketHost(data)
	if err != nil {
		return nil, &ProtocolError{Stage: bootstrapStage, Cause: err}
	}

	topic := valueOrEmpty(data.ApnsTopic)
	if topic == "" {
		return nil, &ProtocolError{Stage: bootstrapStage, Cause: errBootstrap}
	}

	key, err := bootstrapKey(ctx, options.Entropy)
	if err != nil {
		return nil, err
	}

	return retryBootstrap(ctx, host, topic, key, options)
}

func validOptions(options Options) bool {
	return options.Entropy != nil && options.Clock != nil && options.Open != nil &&
		options.Exchange != nil && options.Validate != nil && options.Timeout > 0
}

func retryBootstrap(ctx context.Context, host, topic string, key *ecdsa.PrivateKey, options Options) (*Session, error) {
	var timestamp uint64
	for range bootstrapAttempts {
		session, err := openAttempt(ctx, host, topic, key, timestamp, options)

		var invalidNonce *InvalidNonceError

		if !errors.As(err, &invalidNonce) {
			return session, err
		}

		timestamp = invalidNonce.TimestampMilliseconds
	}

	return nil, &ProtocolError{Stage: bootstrapStage, Cause: errPushToken}
}

func bootstrapKey(ctx context.Context, entropy io.Reader) (*ecdsa.PrivateKey, error) {
	curve := elliptic.P256()

	var scalar *big.Int
	for scalar == nil || scalar.Sign() == 0 {
		contextError := ctx.Err()
		if contextError != nil {
			return nil, &ProtocolError{Stage: stageBootstrapEntropy, Cause: contextError}
		}

		var err error

		scalar, err = rand.Int(entropy, curve.Params().N)
		if err != nil {
			return nil, &ProtocolError{Stage: stageBootstrapEntropy, Cause: err}
		}
	}

	key := new(ecdsa.PrivateKey)
	key.Curve = curve

	private, err := ecdh.P256().NewPrivateKey(scalar.FillBytes(make([]byte, len(curve.Params().N.Bytes()))))
	if err != nil {
		return nil, &ProtocolError{Stage: stageBootstrapKey, Cause: err}
	}

	public := private.PublicKey().Bytes()
	coordinateLength := (len(public) - 1) / int(models.CoordinateParts)
	key.X = new(big.Int).SetBytes(public[1 : 1+coordinateLength])
	key.Y = new(big.Int).SetBytes(public[1+coordinateLength:])
	key.D = scalar

	return key, nil
}

func bootstrapURL(key *ecdsa.PrivateKey, timestamp uint64, options Options, host string) (string, error) {
	if timestamp == 0 {
		timestamp = uint64(options.Clock().UnixMilli())
	}

	nonce := make([]byte, 1+binary.Size(timestamp)+int(bridgepb.Framing_FRAMING_NONCE_ENTROPY_BYTES))
	nonce[0] = byte(bridgepb.Framing_FRAMING_NONCE_VERSION)
	binary.BigEndian.PutUint64(nonce[1:], timestamp)

	_, err := io.ReadFull(options.Entropy, nonce[1+binary.Size(timestamp):])
	if err != nil {
		return "", &ProtocolError{Stage: "nonce entropy", Cause: err}
	}

	digest := sha256.Sum256(nonce)

	signature, err := ecdsa.SignASN1(options.Entropy, key, digest[:])
	if err != nil {
		return "", &ProtocolError{Stage: "bootstrap signing", Cause: err}
	}

	private, err := ecdh.P256().NewPrivateKey(key.D.FillBytes(make([]byte, len(key.Params().N.Bytes()))))
	if err != nil {
		return "", &ProtocolError{Stage: stageBootstrapKey, Cause: err}
	}

	publicKey := private.PublicKey().Bytes()

	message, err := ConnectionMessage(publicKey, nonce, signature)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(string(models.SocketURLTemplate), host, hex.EncodeToString(message)), nil
}

func openAttempt(ctx context.Context, host, topic string, key *ecdsa.PrivateKey,
	timestamp uint64, options Options,
) (*Session, error) {
	socketURL, err := bootstrapURL(key, timestamp, options, host)
	if err != nil {
		return nil, err
	}

	opening, cancelOpening := context.WithTimeout(ctx, options.Timeout)
	socket, err := options.Open(opening, socketURL)

	cancelOpening()

	if err != nil {
		return nil, &ProtocolError{Stage: "open", Cause: err}
	}

	session := new(Session)
	ownerContext, cancel := context.WithCancel(ctx)
	session.ctx, session.cancel = ownerContext, cancel

	session.socket, session.topic, session.options = socket, topic, options

	context.AfterFunc(ownerContext, func() { _ = session.Close() })

	err = session.initialize(socket)
	if err != nil {
		_ = session.Close()

		return nil, err
	}

	err = session.ctx.Err()
	if err != nil {
		_ = session.Close()

		return nil, &ProtocolError{Stage: bootstrapStage, Cause: err}
	}

	return session, nil
}

// Close detaches the connection exactly once, interrupts active I/O, and waits
// for the owned socket to close. Concurrent calls return the same close result.
func (session *Session) Close() error {
	session.closeOnce.Do(func() {
		session.mutex.Lock()
		socket := session.socket
		session.socket = nil
		session.mutex.Unlock()

		if socket == nil {
			return
		}

		session.cancel()

		err := socket.Close()
		if err != nil {
			session.closeError = &ProtocolError{Stage: "close", Cause: err}
		}
	})

	return session.closeError
}

// Active reports whether the connection is attached to a live session owner.
func (session *Session) Active() bool {
	session.mutex.Lock()
	socket := session.socket
	session.mutex.Unlock()

	return socket != nil && session.ctx.Err() == nil
}

func (session *Session) initialize(socket Socket) error {
	token, err := WaitPushToken(session.ctx, socket, session.waitOptions())
	if err != nil {
		return err
	}

	session.token = hex.EncodeToString(token)

	subscription, err := SubscriptionMessage([]string{session.topic})
	if err != nil {
		return err
	}

	err = socket.SendBinary(session.ctx, subscription)
	if err != nil {
		return &ProtocolError{Stage: "subscribe", Cause: err}
	}

	identifier, err := sessionIdentifier(session.options)
	if err != nil {
		return err
	}

	session.identifier = identifier
	exchange := new(models.BridgeExchange)

	exchange.NextStep, exchange.Ptkn, exchange.SessionUUID = models.BootstrapStep, session.token, identifier

	err = session.options.Exchange(session.ctx, *exchange)
	if err != nil {
		return err
	}

	push, err := WaitPush(session.ctx, socket, session.topic, session.waitOptions())
	if err != nil {
		return err
	}

	if push.Payload.SessionUUID != nil && push.SessionID != identifier {
		return errSession
	}

	session.identifier, session.push = push.SessionID, push

	return nil
}

func sessionIdentifier(options Options) (string, error) {
	value := make([]byte, int(bridgepb.Framing_FRAMING_KEY_BYTES))

	_, err := io.ReadFull(options.Entropy, value)
	if err != nil {
		return "", &ProtocolError{Stage: "session entropy", Cause: err}
	}
	// Schema-owned RFC 4122 primitives preserve the pinned session identifier layout.
	value[models.UUIDVersionIndex] =
		value[models.UUIDVersionIndex]&byte(models.UUIDVersionMask) | byte(models.UUIDVersionValue)
	value[models.UUIDVariantIndex] =
		value[models.UUIDVariantIndex]&byte(models.UUIDVariantMask) | byte(models.UUIDVariantValue)

	return fmt.Sprintf(string(models.SessionIdentifierTemplate),
		value[:models.UUIDGroup1End], value[models.UUIDGroup1End:models.UUIDVersionIndex],
		value[models.UUIDVersionIndex:models.UUIDVariantIndex], value[models.UUIDVariantIndex:models.UUIDGroup4End],
		value[models.UUIDGroup4End:], options.Clock().Unix()), nil
}

func (session *Session) waitOptions() WaitOptions {
	return WaitOptions{Timeout: session.options.Timeout, Clock: session.options.Clock}
}
