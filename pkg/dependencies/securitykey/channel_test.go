package securitykey

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
)

func TestRejectOversizedMessage(test *testing.T) {
	test.Parallel()
	channel := &channel{}
	if err := channel.write(test.Context(), byte(wire.CBORCommand), make([]byte, int(wire.MaxPayloadBytes)+1)); !errors.Is(err, ErrProtocol) {
		test.Fatalf("oversized payload accepted: %v", err)
	}
}

func TestRejectContinuationSequence(test *testing.T) {
	test.Parallel()
	initial := make([]byte, int(wire.ReportBytes))
	initial[4] = byte(wire.CBORCommand) | byte(wire.InitialFlag)
	binary.BigEndian.PutUint16(initial[5:], 100)
	continuation := make([]byte, int(wire.ReportBytes))
	continuation[4] = 1
	connection := &fakeConnection{test: test, reads: [][]byte{initial, continuation}}
	channel := &channel{connection: connection}
	if _, err := channel.read(test.Context(), byte(wire.CBORCommand)); !errors.Is(err, ErrProtocol) {
		test.Fatalf("out of order continuation accepted: %v", err)
	}
}

func TestKeepaliveAndUnrelatedChannel(test *testing.T) {
	test.Parallel()
	unrelated := make([]byte, int(wire.ReportBytes))
	unrelated[0] = 1
	keepalive := make([]byte, int(wire.ReportBytes))
	keepalive[4] = byte(wire.KeepaliveCommand) | byte(wire.InitialFlag)
	keepalive[6] = 1
	keepalive[7] = 2
	response := make([]byte, int(wire.ReportBytes))
	response[4] = byte(wire.CBORCommand) | byte(wire.InitialFlag)
	response[6] = 1
	response[7] = 99
	connection := &fakeConnection{test: test, reads: [][]byte{unrelated, keepalive, response}}
	channel := &channel{connection: connection}
	result, err := channel.read(test.Context(), byte(wire.CBORCommand))
	if err != nil || !bytes.Equal(result, []byte{99}) || len(connection.reads) != 0 {
		test.Fatalf("keepalive/unrelated channel handling failed: %x, %v", result, err)
	}
}

func TestContentionWaitCancellation(test *testing.T) {
	test.Parallel()
	fixture := readTranscript(test)
	write := decodeHex(test, fixture.Steps[1].Writes[0])
	response := make([]byte, int(wire.ReportBytes))
	copy(response, []byte{1, 2, 3, 4})
	response[4] = byte(wire.ErrorCommand) | byte(wire.InitialFlag)
	response[6] = 1
	response[7] = byte(wire.ChannelBusy)
	connection := &fakeConnection{test: test, writes: [][]byte{write}, reads: [][]byte{response}}
	waits := 0
	channel := &channel{connection: connection, id: 0x01020304, wait: func(_ context.Context, delay time.Duration) error {
		waits++
		if delay != busyDelay {
			test.Fatal("incorrect contention backoff")
		}
		return context.Canceled
	}}
	_, err := channel.exchange(test.Context(), byte(wire.CBORCommand), []byte{byte(wire.GetInfo)})
	if !errors.Is(err, context.Canceled) || waits != 1 || len(connection.writes) != 0 {
		test.Fatalf("contention cancellation failed: %v", err)
	}
}

func TestProductionBoundaryHonorsCanceledContext(test *testing.T) {
	test.Parallel()
	ctx, cancel := context.WithCancel(test.Context())
	cancel()
	if _, err := (HIDBackend{}).Devices(ctx); !errors.Is(err, context.Canceled) {
		test.Fatalf("discovery cancellation lost: %v", err)
	}
	if _, err := (HIDBackend{}).Open(ctx, "unused"); !errors.Is(err, context.Canceled) {
		test.Fatalf("open cancellation lost: %v", err)
	}
}

func TestRejectInvalidCBOR(test *testing.T) {
	test.Parallel()
	cases := map[string][]byte{"missing status": {}, "noncanonical integer": {0, 0xa1, 0x18, 0x01, 0x00}, "duplicate key": {0, 0xa2, 1, 0, 1, 0}, "indefinite map": {0, 0xbf, 0xff}, "trailing value": {0, 0xa0, 0xa0}, "error status": {byte(wire.NoCredentials)}}
	for name, payload := range cases {
		test.Run(name, func(test *testing.T) {
			test.Parallel()
			fixture := readTranscript(test)
			response := make([]byte, int(wire.ReportBytes))
			copy(response, []byte{1, 2, 3, 4})
			response[4] = byte(wire.CBORCommand) | byte(wire.InitialFlag)
			binary.BigEndian.PutUint16(response[5:], uint16(len(payload)))
			copy(response[7:], payload)
			connection := &fakeConnection{test: test, writes: [][]byte{decodeHex(test, fixture.Steps[1].Writes[0])}, reads: [][]byte{response}}
			channel := &channel{connection: connection, id: 0x01020304}
			var result wire.InfoResponse
			if err := channel.call(test.Context(), wire.GetInfo, nil, &result); err == nil {
				test.Fatal("invalid CBOR accepted")
			}
		})
	}
}
