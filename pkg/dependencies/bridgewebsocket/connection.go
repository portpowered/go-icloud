// Package bridgewebsocket owns the injectable TLS socket and WebSocket framing
// used by the trusted-device bridge. Close interrupts pending reads and sends.
package bridgewebsocket

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6455 mandates SHA-1 solely for the upgrade nonce.
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	bridge "github.com/portpowered/go-icloud/pkg/dependencymodels/bridge"
	pb "github.com/portpowered/go-icloud/pkg/dependencymodels/bridgepb"
	model "github.com/portpowered/go-icloud/pkg/dependencymodels/bridgewebsocket"
)

// closeGrace bounds the optional clean-close attempt before interrupting the socket.
const closeGrace = 100 * time.Millisecond

// ErrProtocol identifies an invalid upgrade or unsupported frame.
var ErrProtocol = errors.New("invalid bridge WebSocket protocol")

// ErrClosed identifies a locally or remotely closed connection.
var ErrClosed = errors.New("bridge WebSocket closed")

// Options supplies one configured endpoint and its caller-owned dependencies.
type Options struct {
	URL       string
	Origin    string
	UserAgent string
	Random    io.Reader
	// Dial returns an already secured connection. Nil uses TLS with certificate verification.
	Dial func(context.Context, string, string) (net.Conn, error)
}

// Conn owns one connection. One read and one send may run concurrently.
type Conn struct {
	socket              net.Conn
	reader              *bufio.Reader
	random              io.Reader
	readTurn, writeTurn chan struct{}
	closed              chan struct{}
	once                sync.Once
}

// Open performs the pinned raw HTTP upgrade and retains coalesced frame bytes.
func Open(ctx context.Context, options Options) (*Conn, error) {
	endpoint, err := validatedEndpoint(options)
	if err != nil {
		return nil, err
	}

	err = ctx.Err()
	if err != nil {
		return nil, fmt.Errorf("bridge upgrade: %w", err)
	}

	if options.Random == nil {
		options.Random = rand.Reader
	}

	keyBytes := make([]byte, int(pb.Framing_FRAMING_KEY_BYTES))

	_, err = io.ReadFull(options.Random, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("bridge upgrade entropy: %w", err)
	}

	dial := options.Dial
	if dial == nil {
		dial = secureDial
	}

	port := endpoint.Port()
	if port == "" {
		port = string(model.SecurePort)
	}

	socket, err := dial(ctx, string(model.DialNetwork), net.JoinHostPort(endpoint.Hostname(), port))
	if err != nil {
		return nil, fmt.Errorf("bridge dial: %w", err)
	}

	conn := new(Conn)
	conn.socket, conn.reader, conn.random = socket, bufio.NewReader(socket), options.Random
	conn.readTurn, conn.writeTurn, conn.closed = make(chan struct{}, 1), make(chan struct{}, 1), make(chan struct{})
	key := base64.StdEncoding.EncodeToString(keyBytes)

	err = conn.upgrade(ctx, endpoint, options, key)
	if err != nil {
		_ = socket.Close()

		return nil, err
	}

	return conn, nil
}

func secureDial(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := new(tls.Dialer)

	connection, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, fmt.Errorf("bridge TLS: %w", err)
	}

	return connection, nil
}

func validatedEndpoint(options Options) (*url.URL, error) {
	endpoint, err := url.Parse(options.URL)
	if err != nil {
		return nil, fmt.Errorf("bridge URL: %w", errors.Join(ErrProtocol, err))
	}

	if endpoint.Scheme != string(model.SecureScheme) || endpoint.Hostname() == "" {
		return nil, ErrProtocol
	}

	if endpoint.User != nil || endpoint.Fragment != "" {
		return nil, ErrProtocol
	}

	for _, value := range []string{options.Origin, options.UserAgent, endpoint.Hostname()} {
		if strings.ContainsAny(value, "\r\n") {
			return nil, ErrProtocol
		}
	}

	return endpoint, nil
}

// Close is idempotent and wakes all owned operations without waiting for a reader.
func (c *Conn) Close() error {
	c.once.Do(func() {
		close(c.closed)

		defer func() { _ = c.socket.Close() }()

		select {
		case c.writeTurn <- struct{}{}:
			defer func() { <-c.writeTurn }()

			_ = c.socket.SetWriteDeadline(time.Now().Add(closeGrace))

			mask := make([]byte, int(pb.Framing_FRAMING_MASK_BYTES))

			_, err := io.ReadFull(c.random, mask)
			if err == nil {
				_ = writeAll(c.socket, encodeFrame(pb.Opcode_OPCODE_CLOSE, nil, mask))
			}
		default:
		}
	})

	return nil
}

func (c *Conn) upgrade(ctx context.Context, endpoint *url.URL, options Options, key string) error {
	release, err := deadline(ctx, c.socket.SetDeadline)
	if err != nil {
		return err
	}

	defer release()

	separator := string([]byte{byte(model.CarriageReturn), byte(model.LineFeed)})

	lines := []string{
		fmt.Sprintf(string(model.UpgradeRequestLine), endpoint.RequestURI()),
		fmt.Sprintf(string(model.HostLine), endpoint.Hostname()),
		string(model.UpgradeLine), string(model.ConnectionLine),
		fmt.Sprintf(string(model.OriginLine), options.Origin),
		fmt.Sprintf(string(model.UserAgentLine), options.UserAgent),
		string(model.VersionLine), fmt.Sprintf(string(model.KeyLine), key), "", "",
	}

	err = writeAll(c.socket, []byte(strings.Join(lines, separator)))
	if err != nil {
		return fmt.Errorf("bridge upgrade write: %w", contextError(ctx, err))
	}

	response, err := http.ReadResponse(c.reader, nil)
	if err != nil {
		return fmt.Errorf("bridge upgrade response: %w", contextError(ctx, err))
	}
	// SHA-1 is required by RFC 6455 solely to authenticate the upgrade nonce.
	//nolint:gosec // RFC 6455 mandates SHA-1 solely for the upgrade nonce.
	digest := sha1.Sum([]byte(key + string(bridge.WebSocketGUID)))

	expected := base64.StdEncoding.EncodeToString(digest[:])

	defer func() { _ = response.Body.Close() }()

	accepted := response.Header.Get(string(model.UpgradeAcceptHeader)) == expected
	if response.StatusCode != http.StatusSwitchingProtocols || !accepted {
		return ErrProtocol
	}

	return nil
}

func deadline(ctx context.Context, set func(time.Time) error) (func(), error) {
	err := ctx.Err()
	if err != nil {
		return nil, fmt.Errorf("bridge operation: %w", err)
	}

	limit, _ := ctx.Deadline()

	err = set(limit)
	if err != nil {
		return nil, fmt.Errorf("bridge deadline: %w", err)
	}

	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { _ = set(time.Now()); close(done) })

	return func() {
		if !stop() {
			<-done
		}

		_ = set(time.Time{})
	}, nil
}

func contextError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return errors.Join(ctx.Err(), err)
	}
	// Socket deadlines can fire before the context timer updates Err.
	limit, bounded := ctx.Deadline()

	var networkError net.Error

	if bounded && !time.Now().Before(limit) && errors.As(err, &networkError) && networkError.Timeout() {
		return errors.Join(context.DeadlineExceeded, err)
	}

	return err
}

func writeAll(writer io.Writer, payload []byte) error {
	for len(payload) > 0 {
		written, err := writer.Write(payload)
		if err != nil {
			return fmt.Errorf("bridge socket write: %w", err)
		}

		if written == 0 {
			return fmt.Errorf("bridge socket write: %w", io.ErrNoProgress)
		}

		payload = payload[written:]
	}

	return nil
}

func (c *Conn) acquire(ctx context.Context, turn chan struct{}) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("bridge operation: %w", ctx.Err())
	case <-c.closed:
		return ErrClosed
	case turn <- struct{}{}:
		return nil
	}
}
