package securitykey

import (
	"context"
	"errors"
	"iter"
	"os"

	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
	"github.com/telesma-app/hid"
)

// HIDBackend uses the operating system's FIDO HID interface without cgo.
type HIDBackend struct {
	enumerate func() iter.Seq2[*hid.DeviceInfo, error]
	open      func(string) (Connection, error)
}

// Devices enumerates FIDO usage-page devices and stops when its context is canceled.
func (backend HIDBackend) Devices(ctx context.Context) ([]Device, error) {
	devices := []Device{}
	if err := ctx.Err(); err != nil {
		return nil, err
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
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		devices = append(devices, Device{ID: info.Path, Name: info.ProductStr})
	}
	return devices, nil
}

// Open creates a handle whose lifecycle belongs to the assertion ceremony.
func (backend HIDBackend) Open(ctx context.Context, id string) (Connection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	open := backend.open
	if open == nil {
		open = func(id string) (Connection, error) { return hid.OpenPath(id) }
	}
	device, err := open(id)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(err, device.Close())
	}
	return device, nil
}
