package securitykey

import (
	"context"
	"errors"
	"iter"
	"os"
	"sync"

	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
	"github.com/telesma-app/hid"
)

// HIDBackend uses the operating system's FIDO HID interface without cgo.
type HIDBackend struct {
	enumerate func() iter.Seq2[*hid.DeviceInfo, error]
	open      func(string) (Connection, error)
}

type ownedConnection struct {
	Connection

	closeOnce    sync.Once
	closeFailure error
}

func (connection *ownedConnection) Close() error {
	connection.closeOnce.Do(func() { connection.closeFailure = connection.Connection.Close() })

	return connection.closeFailure
}

// Devices enumerates FIDO usage-page devices and stops when its context is canceled.
func (backend HIDBackend) Devices(ctx context.Context) ([]Device, error) {
	devices := []Device{}

	err := ctx.Err()
	if err != nil {
		return nil, keyFailure(stageBackend, err)
	}

	enumerate := backend.enumerate
	if enumerate == nil {
		enumerate = func() iter.Seq2[*hid.DeviceInfo, error] {
			return hid.Enumerate(hid.WithUsagePage(uint16(wire.UsagePage)), hid.WithUsage(uint16(wire.Usage)))
		}
	}

	for info, err := range enumerate() {
		if err != nil {
			if errors.Is(err, os.ErrPermission) {
				continue
			}

			return nil, keyFailure(stageBackend, err)
		}

		err := ctx.Err()
		if err != nil {
			return nil, keyFailure(stageBackend, err)
		}

		devices = append(devices, Device{
			ID:   info.Path,
			Name: info.ProductStr,
		})
	}

	return devices, nil
}

// Open creates a handle whose lifecycle belongs to the assertion ceremony.
//
//nolint:ireturn // Backend.Open must return its injectable Connection interface (GO-15).
func (backend HIDBackend) Open(ctx context.Context, deviceID string) (Connection, error) {
	err := ctx.Err()
	if err != nil {
		return nil, keyFailure(stageBackend, err)
	}

	open := backend.open
	if open == nil {
		open = func(deviceID string) (Connection, error) {
			device, err := hid.OpenPath(deviceID)
			if err != nil {
				return nil, keyFailure("HID open", err)
			}

			return device, nil
		}
	}

	device, err := open(deviceID)
	if err != nil {
		return nil, keyFailure(stageBackend, err)
	}

	connection := &ownedConnection{
		Connection:   device,
		closeOnce:    sync.Once{},
		closeFailure: nil,
	}

	err = ctx.Err()
	if err != nil {
		return nil, keyFailure(stageBackend, errors.Join(err, connection.Close()))
	}

	return connection, nil
}
