//nolint:testpackage // White-box protocol and injected HID lifecycle tests require unexported seams (GO-15).
package securitykey

import (
	"bytes"
	"context"
	"errors"
	"iter"
	"os"
	"sync"
	"testing"

	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
	"github.com/telesma-app/hid"
)

func TestDescriptorPermissionFailureSkipped(t *testing.T) {
	t.Parallel()

	backend := HIDBackend{
		enumerate: func() iter.Seq2[*hid.DeviceInfo, error] {
			return func(yield func(*hid.DeviceInfo, error) bool) {
				if !yield(nil, os.ErrPermission) {
					return
				}

				yield(&hid.DeviceInfo{
					Path:           syntheticDeviceID,
					ProductStr:     "Synthetic",
					VendorID:       0,
					ProductID:      0,
					SerialNbr:      "",
					ReleaseNbr:     0,
					MfrStr:         "",
					UsagePage:      0,
					Usage:          0,
					InterfaceNbr:   0,
					InstanceID:     "",
					ParentDeviceID: "",
				}, nil)
			}
		},
		open: nil,
	}

	devices, err := backend.Devices(t.Context())
	if err != nil || len(devices) != 1 || devices[0].ID != syntheticDeviceID {
		t.Fatalf("descriptor permission skip failed: %v, %v", devices, err)
	}
}

func TestDescriptorEnumerationFailurePropagates(t *testing.T) {
	t.Parallel()

	failure := errSyntheticEnumeration
	backend := HIDBackend{
		enumerate: func() iter.Seq2[*hid.DeviceInfo, error] {
			return func(yield func(*hid.DeviceInfo, error) bool) { yield(nil, failure) }
		},
		open: nil,
	}

	_, err := backend.Devices(t.Context())
	if !errors.Is(err, failure) {
		t.Fatalf("enumeration failure lost: %v", err)
	}
}

func TestDiscoveryInitializationFailureClosesHandle(t *testing.T) {
	t.Parallel()
	fixture := readTranscript(t)
	connection := transcriptConnection(t, transcript{
		Steps:             fixture.Steps[:1],
		ClientData:        "",
		AuthenticatorData: "",
		CredentialID:      "",
		CredentialIDs:     nil,
		Signature:         "",
		Error:             "",
	})
	connection.reads[0][7] = 1 // The response no longer echoes the allocation nonce.

	provider, err := New(fakeBackend{
		connection: connection,
		discovery:  nil,
	}, bytes.NewReader([]byte{
		0,
		1,
		2,
		3,
		4,
		5,
		6,
		7,
	}))
	if err != nil {
		t.Fatal(err)
	}

	devices, err := provider.Devices(t.Context())
	if !errors.Is(err, ErrProtocol) || devices != nil || connection.closed != 1 || len(connection.writes) != 0 {
		t.Fatalf("initialization failure did not propagate and close: %v", err)
	}
}

func TestDiscoveryOpenFailurePropagates(t *testing.T) {
	t.Parallel()

	backend := HIDBackend{
		enumerate: func() iter.Seq2[*hid.DeviceInfo, error] {
			return func(yield func(*hid.DeviceInfo, error) bool) {
				yield(&hid.DeviceInfo{
					Path:           syntheticDeviceID,
					VendorID:       0,
					ProductID:      0,
					SerialNbr:      "",
					ReleaseNbr:     0,
					MfrStr:         "",
					ProductStr:     "",
					UsagePage:      0,
					Usage:          0,
					InterfaceNbr:   0,
					InstanceID:     "",
					ParentDeviceID: "",
				}, nil)
			}
		},
		open: func(string) (Connection, error) { return nil, os.ErrPermission },
	}

	provider, err := New(backend, bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}

	_, err = provider.Devices(t.Context())
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("open permission failure was silently skipped: %v", err)
	}
}

func TestDiscoveryCancellationClosesHandle(t *testing.T) {
	t.Parallel()
	fixture := readTranscript(t)
	connection := transcriptConnection(t, transcript{
		Steps:             fixture.Steps[:1],
		ClientData:        "",
		AuthenticatorData: "",
		CredentialID:      "",
		CredentialIDs:     nil,
		Signature:         "",
		Error:             "",
	})
	cancelPacket := make([]byte, int(wire.ReportBytes)+1)
	copy(cancelPacket[1:], []byte{
		255,
		255,
		255,
		255,
	})

	cancelPacket[5] = byte(wire.CancelCommand) | byte(wire.InitialFlag)
	connection.writes = append(connection.writes, cancelPacket)
	connection.cancelOnRead = true

	provider, err := New(fakeBackend{
		connection: connection,
		discovery:  nil,
	}, bytes.NewReader([]byte{
		0,
		1,
		2,
		3,
		4,
		5,
		6,
		7,
	}))
	if err != nil {
		t.Fatal(err)
	}

	_, err = provider.Devices(t.Context())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("discovery cancellation lost: %v", err)
	}

	if connection.closed != 1 || len(connection.writes) != 0 {
		t.Fatal("discovery did not cancel before closing exactly once")
	}
}

func TestOpenCancellationClosesNewHandle(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	connection := &fakeConnection{
		t:                 t,
		writes:            nil,
		reads:             nil,
		closed:            0,
		cancelOnRead:      false,
		cancelAfterReads:  0,
		readCount:         0,
		readFailure:       nil,
		failureAfterReads: 0,
		closeFailure:      nil,
	}
	backend := HIDBackend{
		open: func(string) (Connection, error) {
			cancel()
			return connection, nil
		},
		enumerate: nil,
	}

	_, err := backend.Open(ctx, syntheticDeviceID)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("open cancellation lost: %v", err)
	}

	if connection.closed != 1 {
		t.Fatal("canceled new handle was not closed")
	}
}

func TestProductionHandleCloseIsIdempotent(t *testing.T) {
	t.Parallel()

	failure := errSyntheticClose
	physical := &fakeConnection{
		t: t, writes: nil, reads: nil, closed: 0, cancelOnRead: false,
		cancelAfterReads: 0, readCount: 0, readFailure: nil,
		failureAfterReads: 0, closeFailure: failure,
	}
	backend := HIDBackend{
		enumerate: nil,
		open:      func(string) (Connection, error) { return physical, nil },
	}

	connection, err := backend.Open(t.Context(), syntheticDeviceID)
	if err != nil {
		t.Fatal(err)
	}

	var workers sync.WaitGroup
	for range 2 {
		workers.Go(func() {
			err := connection.Close()
			if !errors.Is(err, failure) {
				t.Errorf("close failure lost: %v", err)
			}
		})
	}

	workers.Wait()

	if physical.closed != 1 {
		t.Fatalf("physical handle closed %d times", physical.closed)
	}
}
