package securitykey

import (
	"bytes"
	"context"
	"errors"
	"iter"
	"os"
	"testing"

	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
	"github.com/telesma-app/hid"
)

func TestDescriptorPermissionFailureSkipped(test *testing.T) {
	test.Parallel()
	backend := HIDBackend{enumerate: func() iter.Seq2[*hid.DeviceInfo, error] {
		return func(yield func(*hid.DeviceInfo, error) bool) {
			if !yield(nil, os.ErrPermission) {
				return
			}
			yield(&hid.DeviceInfo{Path: "synthetic", ProductStr: "Synthetic"}, nil)
		}
	}}
	devices, err := backend.Devices(test.Context())
	if err != nil || len(devices) != 1 || devices[0].ID != "synthetic" {
		test.Fatalf("descriptor permission skip failed: %v, %v", devices, err)
	}
}

func TestDescriptorEnumerationFailurePropagates(test *testing.T) {
	test.Parallel()
	failure := errors.New("synthetic enumeration failure")
	backend := HIDBackend{enumerate: func() iter.Seq2[*hid.DeviceInfo, error] {
		return func(yield func(*hid.DeviceInfo, error) bool) { yield(nil, failure) }
	}}
	if _, err := backend.Devices(test.Context()); !errors.Is(err, failure) {
		test.Fatalf("enumeration failure lost: %v", err)
	}
}

func TestDiscoveryInitializationFailureClosesHandle(test *testing.T) {
	test.Parallel()
	fixture := readTranscript(test)
	connection := transcriptConnection(test, transcript{Steps: fixture.Steps[:1]})
	connection.reads[0][7] = 1 // The response no longer echoes the allocation nonce.
	provider, err := New(fakeBackend{connection: connection}, bytes.NewReader([]byte{0, 1, 2, 3, 4, 5, 6, 7}))
	if err != nil {
		test.Fatal(err)
	}
	devices, err := provider.Devices(test.Context())
	if !errors.Is(err, ErrProtocol) || devices != nil || connection.closed != 1 || len(connection.writes) != 0 {
		test.Fatalf("initialization failure did not propagate and close: %v", err)
	}
}

func TestDiscoveryOpenFailurePropagates(test *testing.T) {
	test.Parallel()
	backend := HIDBackend{enumerate: func() iter.Seq2[*hid.DeviceInfo, error] {
		return func(yield func(*hid.DeviceInfo, error) bool) { yield(&hid.DeviceInfo{Path: "synthetic"}, nil) }
	}, open: func(string) (Connection, error) { return nil, os.ErrPermission }}
	provider, err := New(backend, bytes.NewReader(nil))
	if err != nil {
		test.Fatal(err)
	}
	if _, err := provider.Devices(test.Context()); !errors.Is(err, os.ErrPermission) {
		test.Fatalf("open permission failure was silently skipped: %v", err)
	}
}

func TestDiscoveryCancellationClosesHandle(test *testing.T) {
	test.Parallel()
	fixture := readTranscript(test)
	connection := transcriptConnection(test, transcript{Steps: fixture.Steps[:1]})
	cancelPacket := make([]byte, int(wire.ReportBytes)+1)
	copy(cancelPacket[1:], []byte{255, 255, 255, 255})
	cancelPacket[5] = byte(wire.CancelCommand) | byte(wire.InitialFlag)
	connection.writes = append(connection.writes, cancelPacket)
	connection.cancelOnRead = true
	provider, err := New(fakeBackend{connection: connection}, bytes.NewReader([]byte{0, 1, 2, 3, 4, 5, 6, 7}))
	if err != nil {
		test.Fatal(err)
	}
	if _, err := provider.Devices(test.Context()); !errors.Is(err, context.Canceled) {
		test.Fatalf("discovery cancellation lost: %v", err)
	}
	if connection.closed != 1 || len(connection.writes) != 0 {
		test.Fatal("discovery did not cancel before closing exactly once")
	}
}

func TestOpenCancellationClosesNewHandle(test *testing.T) {
	test.Parallel()
	ctx, cancel := context.WithCancel(test.Context())
	defer cancel()
	connection := &fakeConnection{test: test}
	backend := HIDBackend{open: func(string) (Connection, error) { cancel(); return connection, nil }}
	if _, err := backend.Open(ctx, "synthetic"); !errors.Is(err, context.Canceled) {
		test.Fatalf("open cancellation lost: %v", err)
	}
	if connection.closed != 1 {
		test.Fatal("canceled new handle was not closed")
	}
}
