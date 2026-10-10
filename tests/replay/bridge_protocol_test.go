package replay_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/dependencies/bridge"
	"github.com/portpowered/go-icloud/pkg/dependencies/bridgewebsocket"
)

type bridgeProtocolAction struct {
	Operation      string    `json:"operation"`
	Timeout        float64   `json:"timeout"`
	MonotonicTicks []float64 `json:"monotonic_ticks"` //nolint:tagliatelle // The Source transcript owns this field.
	Topics         []string  `json:"topics"`
	Topic          string    `json:"topic"`
}

type bridgeProtocolFixture struct {
	Actions []bridgeProtocolAction `json:"actions"`
}

func TestBridgeProtocolPairedTranscripts(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(replayLiteralFixturesSyntheticSocketJSON)
	if err != nil {
		t.Fatal(err)
	}

	covered := 0

	for _, path := range paths {
		fixture := readBridgeSocketFixture(t, path)
		if rawBridgeSocketFixture(fixture) {
			continue
		}

		covered++

		t.Run(fixture.Operation, func(t *testing.T) {
			t.Parallel()
			runBridgeProtocol(t, path, fixture)
		})
	}

	if covered != 35 {
		t.Fatalf("protocol transcript inventory changed: %d", covered)
	}
}

func runBridgeProtocol(t *testing.T, path string, fixture bridgeSocketFixture) {
	t.Helper()

	//nolint:gosec // The path comes from the fixed canonical fixture glob.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var actions bridgeProtocolFixture

	err = json.Unmarshal(data, &actions)
	if err != nil {
		t.Fatal(err)
	}

	script := new(bridgeScriptSocket)
	script.events = fixture.Events
	options := new(bridgewebsocket.Options)
	options.URL = fixture.Connection.URL
	options.Origin, options.UserAgent = fixture.Connection.Origin, fixture.Connection.UserAgent
	options.Random = bytes.NewReader(make([]byte, 1024))
	options.Dial = func(context.Context, string, string) (net.Conn, error) { return script, nil }

	socket, err := bridgewebsocket.Open(t.Context(), *options)
	if err != nil {
		t.Fatal(err)
	}

	results, operationError := executeBridgeProtocol(t.Context(), socket, actions.Actions)

	err = socket.Close()
	if err != nil {
		t.Fatal(err)
	}

	if (operationError != nil) != (len(fixture.Error) != 0) {
		t.Fatalf("operation error = %v; expected = %s", operationError, fixture.Error)
	}

	assertBridgeProtocolResults(t, results, fixture.Results)

	if fixture.Operation == "token-invalid-nonce" {
		var nonce *bridge.InvalidNonceError
		if !errors.As(operationError, &nonce) || nonce.TimestampMilliseconds != 123000 {
			t.Fatalf("nonce evidence missing: %v", operationError)
		}
	}

	assertBridgeSocketConsumed(t, script)
}

func assertBridgeSocketConsumed(t *testing.T, script *bridgeScriptSocket) {
	t.Helper()
	script.mu.Lock()
	defer script.mu.Unlock()

	if script.failure != nil {
		t.Fatal(script.failure)
	}

	if script.cursor != len(script.events) || !script.closed {
		t.Fatalf("incomplete socket transcript %d/%d, closed=%t", script.cursor, len(script.events), script.closed)
	}
}

func executeBridgeProtocol(ctx context.Context, socket bridge.Socket, actions []bridgeProtocolAction) ([]any, error) {
	results := []any{}

	for _, action := range actions {
		options := bridge.WaitOptions{
			Timeout: time.Duration(action.Timeout * float64(time.Second)),
			Clock:   fixtureBridgeClock(action.MonotonicTicks),
		}

		switch action.Operation {
		case "wait_push_token":
			value, err := bridge.WaitPushToken(ctx, socket, options)
			if err != nil {
				return results, fmt.Errorf(replaySocketOperationErrorFormat, err)
			}

			results = append(results, base64.StdEncoding.EncodeToString(value))
		case "subscribe":
			value, err := bridge.SubscriptionMessage(action.Topics)
			if err != nil {
				return results, fmt.Errorf(replaySocketOperationErrorFormat, err)
			}

			err = socket.SendBinary(ctx, value)
			if err != nil {
				return results, fmt.Errorf(replaySocketOperationErrorFormat, err)
			}
		case "wait_push":
			push, err := bridge.WaitPush(ctx, socket, action.Topic, options)
			if err != nil {
				return results, fmt.Errorf(replaySocketOperationErrorFormat, err)
			}

			results = append(results, sourceBridgePush(push))
		}
	}

	return results, nil
}

func fixtureBridgeClock(ticks []float64) func() time.Time {
	cursor := 0

	return func() time.Time {
		if cursor >= len(ticks) {
			return time.Unix(0, 0).Add(time.Hour)
		}

		tick := ticks[cursor]
		cursor++

		return time.Unix(0, int64(tick*float64(time.Second)))
	}
}

func sourceBridgePush(push *bridge.Push) map[string]any {
	result := map[string]any{
		"payload": push.Payload, "session_uuid": push.SessionID, replayLiteralNextStep: nil,
		"rui_url_key": push.Payload.RuiURLKey, "txnid": push.Payload.Txnid,
		"salt": push.Payload.Salt, "mid": push.Payload.Mid, "idmsdata": push.Payload.Idmsdata,
		"akdata": push.Payload.Akdata, "data": push.Payload.Data,
		"encrypted_code": push.Payload.EncryptedCode, "error_code": push.Payload.Ec,
	}
	if push.NextStep != "" {
		result[replayLiteralNextStep] = push.NextStep
	}

	return result
}

func assertBridgeProtocolResults(t *testing.T, results []any, expected json.RawMessage) {
	t.Helper()

	encoded, err := json.Marshal(results)
	if err != nil {
		t.Fatal(err)
	}

	var actual, wanted any

	err = json.Unmarshal(encoded, &actual)
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(expected, &wanted)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(actual, wanted) {
		t.Fatalf("result=%s; want=%s", encoded, expected)
	}
}
