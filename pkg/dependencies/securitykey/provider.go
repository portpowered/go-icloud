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

const stageCTAP = "CTAP"
const stageHID = "HID"
const stageClientData = "client data"
const stageBackend = "HID backend"

// Device identifies one discoverable authenticator.
type Device struct {
	ID   string
	Name string
}

// Connection owns an opened HID handle. Read and Write must honor cancellation.
type Connection interface {
	Read(ctx context.Context, data []byte) (int, error)
	Write(ctx context.Context, data []byte) (int, error)
	Close() error
}

// Backend discovers authenticators and opens a handle owned by its caller.
type Backend interface {
	Devices(ctx context.Context) ([]Device, error)
	Open(ctx context.Context, id string) (Connection, error)
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
			return keyFailure("wait configuration", ErrProtocol)
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
		return keyFailure("wait", ctx.Err())
	case <-timer.C:
		return nil
	}
}

// New returns a provider using explicit concurrency-safe hardware and entropy boundaries.
func New(backend Backend, entropy io.Reader, options ...Option) (*Provider, error) {
	if backend == nil || entropy == nil {
		return nil, keyFailure("configuration", ErrProtocol)
	}

	provider := &Provider{
		backend: backend,
		entropy: entropy,
		wait:    waitContext,
	}

	for _, option := range options {
		if option == nil {
			return nil, keyFailure("configuration", ErrProtocol)
		}

		err := option(provider)
		if err != nil {
			return nil, err
		}
	}

	return provider, nil
}

// Devices returns the current device inventory without retaining open handles.
func (provider *Provider) Devices(ctx context.Context) ([]Device, error) {
	devices, err := provider.backend.Devices(ctx)
	if err != nil {
		return nil, keyFailure("discovery", err)
	}

	result := make([]Device, 0, len(devices))

	for _, device := range devices {
		err := provider.inspectDevice(ctx, device.ID)
		if err != nil {
			return nil, err
		}

		result = append(result, device)
	}

	return result, nil
}

// Assert opens the selected device and releases it after the assertion or failure.
func (provider *Provider) Assert(ctx context.Context, request Request) (Assertion, error) {
	if request.DeviceID == "" || request.RelyingPartyID == "" || request.Origin == "" {
		return emptyAssertion(), keyFailure("request", ErrProtocol)
	}

	connection, err := provider.backend.Open(ctx, request.DeviceID)
	if err != nil {
		return emptyAssertion(), keyFailure("open", err)
	}

	result, failure := provider.assertConnection(ctx, connection, request)

	closeErr := connection.Close()

	if closeErr != nil {
		failure = errors.Join(failure, keyFailure("close", closeErr))
	}

	return result, failure
}

func (provider *Provider) inspectDevice(ctx context.Context, id string) error {
	connection, err := provider.backend.Open(ctx, id)
	if err != nil {
		return keyFailure("discovery open", err)
	}

	_, failure := provider.initializeConnection(ctx, connection)

	closeErr := connection.Close()

	if closeErr != nil {
		return keyFailure("discovery close", errors.Join(failure, closeErr))
	}

	return failure
}

func (provider *Provider) initializeConnection(ctx context.Context, connection Connection) (*channel, error) {
	nonce := make([]byte, int(wire.NonceBytes))

	_, err := io.ReadFull(provider.entropy, nonce)
	if err != nil {
		return nil, keyFailure("entropy", err)
	}

	transport := &channel{
		connection:      connection,
		id:              uint32(wire.BroadcastChannel),
		ctap2:           false,
		wait:            provider.wait,
		entropy:         provider.entropy,
		maxMessageBytes: 0,
	}

	err = transport.initialize(ctx, nonce)
	if err != nil {
		return nil, err
	}

	return transport, nil
}

func (provider *Provider) assertConnection(
	ctx context.Context, connection Connection, request Request,
) (Assertion, error) {
	transport, err := provider.initializeConnection(ctx, connection)
	if err != nil {
		return emptyAssertion(), err
	}

	if transport.ctap2 {
		return transport.assert(ctx, request)
	}

	return transport.assertU2F(ctx, request)
}

func emptyAssertion() Assertion {
	var result Assertion
	return result
}

func keyFailure(stage string, cause error) *Error {
	return &Error{Stage: stage, Status: 0, Cause: cause}
}
