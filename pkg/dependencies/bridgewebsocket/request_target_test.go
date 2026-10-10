package bridgewebsocket_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencies/bridgewebsocket"
)

func TestBridgeRequestTargetRejectsBeforeDial(t *testing.T) {
	t.Parallel()

	for _, target := range []string{
		"/v2/", "/other/abcd", "/v2/ABCD", "/v2/abcd/extra", "/v2/%61bcd",
		"/v2/abcd?channel=1", "/v2/abcd?", "/v2/abcd%2fextra", "/v2/abcd%3fchannel=1",
	} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			options := new(bridgewebsocket.Options)
			options.URL = "wss://bridge.example.invalid" + target
			options.Dial = func(context.Context, string, string) (net.Conn, error) {
				t.Fatal("invalid request target reached the injected dialer")

				return nil, bridgewebsocket.ErrProtocol
			}

			_, err := bridgewebsocket.Open(t.Context(), *options)

			if !errors.Is(err, bridgewebsocket.ErrProtocol) {
				t.Fatalf("invalid target error=%v", err)
			}
		})
	}
}
