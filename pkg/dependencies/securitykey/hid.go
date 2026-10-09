package securitykey

import (
	"context"
	"errors"

	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
	"github.com/telesma-app/hid"
)

// HIDBackend uses the operating system's FIDO HID interface without cgo.
type HIDBackend struct{}

// Devices enumerates FIDO usage-page devices and stops when its context is canceled.
func (HIDBackend) Devices(ctx context.Context) ([]Device, error) {
	devices := []Device{}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for info, err := range hid.Enumerate(hid.WithUsagePage(uint16(wire.UsagePage)), hid.WithUsage(uint16(wire.Usage))) {
		if err != nil {
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
func (HIDBackend) Open(ctx context.Context, id string) (Connection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	device, err := hid.OpenPath(id)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(err, device.Close())
	}
	return device, nil
}
