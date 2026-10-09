// Package securitykey implements cancellable FIDO authenticator assertions over HID.
package securitykey

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
)

// Device identifies one discoverable authenticator.
type Device struct {
	ID   string
	Name string
}

// Connection owns an opened HID handle. Read and Write must honor cancellation.
type Connection interface {
	Read(context.Context, []byte) (int, error)
	Write(context.Context, []byte) (int, error)
	Close() error
}

// Backend discovers authenticators and opens a handle owned by its caller.
type Backend interface {
	Devices(context.Context) ([]Device, error)
	Open(context.Context, string) (Connection, error)
}

// Request binds an assertion to a device, relying party, challenge and origin.
type Request struct {
	DeviceID       string
	RelyingPartyID string
	Challenge      string
	Origin         string
	CredentialIDs  []string
}

// Assertion contains the authenticator's signed proof and associated client data.
type Assertion struct {
	AuthenticatorData []byte
	ClientData        []byte
	CredentialID      []byte
	Signature         []byte
	UserHandle        []byte
}

// Error preserves a device failure without including credentials or message contents.
type Error struct {
	Stage  string
	Status int
	Cause  error
}

func (err *Error) Error() string {
	return fmt.Sprintf("security key %s failed (status %d)", err.Stage, err.Status)
}
func (err *Error) Unwrap() error { return err.Cause }

var (
	// ErrProtocol indicates an invalid or inconsistent authenticator message.
	ErrProtocol = errors.New("invalid security key protocol message")
	// ErrUnsupported indicates a capability unavailable through this ceremony.
	ErrUnsupported = errors.New("security key capability unsupported")
	// ErrPINRequired matches the source's default interaction, which cannot supply a PIN.
	ErrPINRequired = errors.New("security key PIN required")
)

// Provider keeps hardware access injectable and creates a fresh handle for each assertion.
type Provider struct {
	backend Backend
	entropy io.Reader
	wait    Waiter
}

// Waiter supplies cancellable retry timing for HID contention and U2F presence polling.
type Waiter func(context.Context, time.Duration) error

// Option configures a provider's lifecycle boundaries.
type Option func(*Provider) error

// WithWaiter injects retry timing without changing the protocol polling policy.
func WithWaiter(wait Waiter) Option {
	return func(provider *Provider) error {
		if wait == nil {
			return &Error{Stage: "wait configuration", Cause: ErrProtocol}
		}
		provider.wait = wait
		return nil
	}
}

func waitContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// New returns a provider using explicit concurrency-safe hardware and entropy boundaries.
func New(backend Backend, entropy io.Reader, options ...Option) (*Provider, error) {
	if backend == nil || entropy == nil {
		return nil, &Error{Stage: "configuration", Cause: ErrProtocol}
	}
	provider := &Provider{backend: backend, entropy: entropy, wait: waitContext}
	for _, option := range options {
		if option == nil {
			return nil, &Error{Stage: "configuration", Cause: ErrProtocol}
		}
		if err := option(provider); err != nil {
			return nil, err
		}
	}
	return provider, nil
}

// Devices returns the current device inventory without retaining open handles.
func (provider *Provider) Devices(ctx context.Context) ([]Device, error) {
	devices, err := provider.backend.Devices(ctx)
	if err != nil {
		return nil, &Error{Stage: "discovery", Cause: err}
	}
	result := make([]Device, 0, len(devices))
	for _, device := range devices {
		if err := provider.inspectDevice(ctx, device.ID); err != nil {
			return nil, err
		}
		result = append(result, device)
	}
	return result, nil
}

func (provider *Provider) inspectDevice(ctx context.Context, id string) (failure error) {
	connection, err := provider.backend.Open(ctx, id)
	if err != nil {
		return &Error{Stage: "discovery open", Cause: err}
	}
	defer func() {
		if err := connection.Close(); err != nil {
			failure = &Error{Stage: "discovery close", Cause: errors.Join(failure, err)}
		}
	}()
	nonce := make([]byte, int(wire.NonceBytes))
	if _, err := io.ReadFull(provider.entropy, nonce); err != nil {
		return &Error{Stage: "discovery entropy", Cause: err}
	}
	transport := &channel{connection: connection, id: uint32(wire.BroadcastChannel), wait: provider.wait, entropy: provider.entropy}
	return transport.initialize(ctx, nonce)
}

// Assert opens the selected device and releases it after the assertion or failure.
func (provider *Provider) Assert(ctx context.Context, request Request) (result Assertion, failure error) {
	if request.DeviceID == "" || request.RelyingPartyID == "" || request.Origin == "" {
		return result, &Error{Stage: "request", Cause: ErrProtocol}
	}
	connection, err := provider.backend.Open(ctx, request.DeviceID)
	if err != nil {
		return result, &Error{Stage: "open", Cause: err}
	}
	defer func() {
		if err := connection.Close(); err != nil {
			failure = errors.Join(failure, &Error{Stage: "close", Cause: err})
		}
	}()
	nonce := make([]byte, int(wire.NonceBytes))
	_, err = io.ReadFull(provider.entropy, nonce)
	if err != nil {
		return result, &Error{Stage: "entropy", Cause: err}
	}
	transport := &channel{connection: connection, id: uint32(wire.BroadcastChannel), wait: provider.wait, entropy: provider.entropy}
	if err := transport.initialize(ctx, nonce); err != nil {
		return result, err
	}
	if transport.ctap2 {
		return transport.assert(ctx, request)
	}
	return transport.assertU2F(ctx, request)
}
