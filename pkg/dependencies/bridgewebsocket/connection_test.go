package bridgewebsocket_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // RFC 6455 requires SHA-1 for its upgrade nonce.
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/portpowered/go-icloud/pkg/dependencies/bridgewebsocket"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// The live pipe exercises owned I/O cancellation rather than fake deadlines.
func openedBridgePipe(t *testing.T) (*bridgewebsocket.Conn, net.Conn) {
	t.Helper()

	client, server := net.Pipe()

	t.Cleanup(func() { _ = server.Close(); _ = client.Close() })

	ready := make(chan error, 1)

	go func() {
		reader := bufio.NewReader(server)

		request, err := http.ReadRequest(reader)
		if err != nil {
			ready <- err

			return
		}

		key := request.Header.Get("Sec-WebSocket-Key")
		//nolint:gosec // RFC 6455 specifies SHA-1 for its upgrade nonce.
		digest := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		response := "HTTP/1.1 101 Switching Protocols\r\nSec-WebSocket-Accept: " +
			base64.StdEncoding.EncodeToString(digest[:]) + "\r\n\r\n"

		_, err = io.WriteString(server, response)
		ready <- err
	}()

	options := new(bridgewebsocket.Options)
	options.URL = "wss://bridge.example.invalid/v2/abcdef0123456789"
	options.Origin, options.UserAgent = "https://www.icloud.com", "test"
	options.Random = bytes.NewReader(make([]byte, 256))
	options.Dial = func(context.Context, string, string) (net.Conn, error) { return client, nil }

	conn, err := bridgewebsocket.Open(t.Context(), *options)
	if err != nil {
		t.Fatal(err)
	}

	err = <-ready
	if err != nil {
		t.Fatal(err)
	}

	return conn, server
}

func TestBridgeWebSocketCanceledReadAndSend(t *testing.T) {
	t.Parallel()

	for _, send := range []bool{false, true} {
		t.Run(fmt.Sprintf("send-%t", send), func(t *testing.T) {
			t.Parallel()
			conn, _ := openedBridgePipe(t)

			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
			defer cancel()

			var err error
			if send {
				err = conn.SendBinary(ctx, []byte("blocked"))
			} else {
				_, err = conn.ReadMessage(ctx)
			}

			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("cancellation error = %v", err)
			}

			err = conn.Close()
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBridgeWebSocketConcurrentClose(t *testing.T) {
	t.Parallel()
	conn, server := openedBridgePipe(t)
	readDone, sendDone := make(chan error, 1), make(chan error, 1)

	go func() { _, err := conn.ReadMessage(t.Context()); readDone <- err }()
	go func() { sendDone <- conn.SendBinary(t.Context(), []byte("pending")) }()
	// Concurrent Close calls must complete even if the peer stops consuming bytes.
	closeDone := make(chan error, 2)
	go func() { closeDone <- conn.Close() }()
	go func() { closeDone <- conn.Close() }()

	for range 2 {
		err := <-closeDone
		if err != nil {
			t.Fatal(err)
		}
	}

	err := <-readDone
	if err == nil {
		t.Fatal("read survived close")
	}

	err = <-sendDone
	if err == nil {
		t.Fatal("send survived close")
	}

	err = server.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestBridgeWebSocketRejectsUnsafeUpgradeInput(t *testing.T) {
	t.Parallel()

	invalid := []string{
		"https://bridge.example.invalid/v2/x", "wss:///v2/x",
		"wss://user:password@bridge.example.invalid/v2/x", "wss://bridge.example.invalid/v2/x#fragment",
	}
	for _, endpoint := range invalid {
		options := new(bridgewebsocket.Options)
		options.URL = endpoint
		options.Dial = func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("invalid upgrade attempted dial")

			return nil, io.ErrClosedPipe
		}

		_, err := bridgewebsocket.Open(t.Context(), *options)
		if !errors.Is(err, bridgewebsocket.ErrProtocol) {
			t.Fatalf("URL %q error %v", endpoint, err)
		}
	}
}
