package icloud

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/bridge"
	"github.com/portpowered/go-icloud/pkg/dependencies/bridgewebsocket"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
	bridgeModels "github.com/portpowered/go-icloud/pkg/dependencymodels/bridge"
)

const nativeBridgeTimeout = 30 * time.Second

var errNativeBridgeOption = errors.New("native bridge option or secure dial hook is nil")

// NativeBridgeDial returns a secured connection; tests can supply an offline socket.
type NativeBridgeDial func(ctx context.Context, network, address string) (net.Conn, error)

type nativeBridgeConfiguration struct {
	dial NativeBridgeDial
}

// NativeBridgeOption configures one account-bound socket lifecycle.
type NativeBridgeOption func(*nativeBridgeConfiguration) error

// WithNativeBridgeDial injects the connection-producing network edge for this session.
func WithNativeBridgeDial(dial NativeBridgeDial) NativeBridgeOption {
	return func(configuration *nativeBridgeConfiguration) error {
		if dial == nil {
			return errNativeBridgeOption
		}
		configuration.dial = dial
		return nil
	}
}

// NativeBridgeSession owns caller-visible credentials and one trusted-device socket.
// VerifyCode closes the socket, including failure paths, and returns current credentials.
type NativeBridgeSession struct {
	mutex      sync.Mutex
	sdk        *SDK
	connection *bridge.Session
	state      NativeAuthState
	responses  []ResponseMetadata
	busy       atomic.Bool
}

// OpenNativeBridgeSession bootstraps the native prompt after GetAuthenticationChallenge.
// Failures after bootstrap starts return a closed session that retains credential progress.
// Inspect that session's State before choosing another delivery route.
func (sdk *SDK) OpenNativeBridgeSession(ctx context.Context, request OpenNativeBridgeSessionRequest,
	options ...NativeBridgeOption,
) (*NativeBridgeSession, error) {
	const operation = "OpenNativeBridgeSession"

	progress, err := newNativeAuthOperation(ctx, operation, request.Auth, request.State)
	if err != nil {
		return nil, err
	}
	configuration, err := nativeBridgeConfig(options)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	var boot bridgeModels.BridgeBootstrapDirect
	if err := json.Unmarshal(progress.state.Challenge.BridgeBootstrap, &boot); err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}
	if boot.TwoSV == nil || boot.TwoSV.BridgeInitiateData == nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, errNativeAuthInput)
	}
	owner := new(NativeBridgeSession)
	owner.sdk, owner.state, owner.responses = sdk, progress.state, []ResponseMetadata{}
	connection, err := bridge.Start(ctx, *boot.TwoSV.BridgeInitiateData,
		owner.bridgeOptions(configuration))
	if err != nil {
		failure := owner.failure(operation, err)
		owner.bridgeFallbackNotice(err)

		return owner, failure
	}
	owner.connection = connection
	owner.state.CodeRequested = true
	owner.state.DeliveryMethod = TwoFactorDeliveryTrustedDevice
	return owner, nil
}

// State returns copied credentials, response metadata, and the current socket challenge.
func (session *NativeBridgeSession) State() (*NativeBridgeSessionState, error) {
	if session.connection == nil {
		session.mutex.Lock()
		state := cloneNativeAuthState(session.state)
		responses := cloneDriveResponses(session.responses)
		session.mutex.Unlock()

		return &NativeBridgeSessionState{State: state, Responses: responses,
			Active: false, Legacy: false, SessionID: "", NextStep: "",
			TransactionID: nullable.NewNullNullable[string]()}, nil
	}

	push, err := session.connection.Snapshot()
	if err != nil {
		return nil, session.failure("NativeBridgeSession.State", err)
	}

	session.mutex.Lock()
	state, responses := cloneNativeAuthState(session.state), cloneDriveResponses(session.responses)
	session.mutex.Unlock()
	transaction := nullable.NewNullNullable[string]()
	if push.Payload.Txnid != nil {
		transaction = nullable.NewNullableWithValue(*push.Payload.Txnid)
	}
	return &NativeBridgeSessionState{State: state, Responses: responses,
		Active: session.connection.Active(), Legacy: bridgeLegacy(push),
		SessionID: push.SessionID, NextStep: push.NextStep, TransactionID: transaction}, nil
}

// VerifyCode completes modern bridge proof or its selected legacy verifier.
func (session *NativeBridgeSession) VerifyCode(ctx context.Context,
	request VerifyNativeBridgeCodeRequest,
) (*NativeAuthResult, error) {
	const operation = "NativeBridgeSession.VerifyCode"
	if !session.busy.CompareAndSwap(false, true) {
		return nil, session.failure(operation, bridge.ErrBusy)
	}

	defer session.busy.Store(false)
	defer func() { _ = session.Close() }()
	if session.connection == nil || !session.connection.Active() {
		return nil, session.failure(operation, bridge.ErrClosed)
	}
	push, err := session.connection.Snapshot()
	if err != nil {
		return nil, session.failure(operation, err)
	}

	session.mutex.Lock()
	session.state.CodeRequested = false
	state := cloneNativeAuthState(session.state)
	session.mutex.Unlock()
	if bridgeLegacy(push) {
		return session.verifyLegacy(ctx, state, request.Code)
	}
	verified, err := session.connection.VerifyCode(ctx, request.Code)
	if err != nil {
		return nil, session.failure(operation, err)
	}
	if !verified {
		return session.result(false)
	}

	session.mutex.Lock()
	session.state.RequiresMFA = false
	state = cloneNativeAuthState(session.state)
	session.mutex.Unlock()
	result, err := session.sdk.TrustSession(ctx, NativeAuthRequest{Auth: state.Auth, State: state})
	if err != nil {
		return nil, session.failure(operation, err)
	}

	session.recordResult(result)
	return session.result(result.Success)
}

// Close is idempotent and interrupts an in-flight verification.
func (session *NativeBridgeSession) Close() error {
	if session.connection == nil {
		return nil
	}

	if err := session.connection.Close(); err != nil {
		return session.failure("NativeBridgeSession.Close", err)
	}
	return nil
}

func nativeBridgeConfig(options []NativeBridgeOption) (*nativeBridgeConfiguration, error) {
	configuration := new(nativeBridgeConfiguration)

	for _, option := range options {
		if option == nil {
			return nil, errNativeBridgeOption
		}
		if err := option(configuration); err != nil {
			return nil, err
		}
	}
	return configuration, nil
}

func (session *NativeBridgeSession) bridgeOptions(configuration *nativeBridgeConfiguration) bridge.Options {
	options := new(bridge.Options)
	options.Entropy, options.Clock, options.Timeout = session.sdk.random, session.sdk.clock, nativeBridgeTimeout
	options.Open = func(ctx context.Context, endpoint string) (bridge.Socket, error) {
		input := new(bridgewebsocket.Options)
		input.URL, input.Random = endpoint, session.sdk.random
		input.Dial = configuration.dial

		session.mutex.Lock()
		state := cloneNativeAuthState(session.state)
		session.mutex.Unlock()
		input.Origin = nativeIDMSOrigin(state)
		input.UserAgent = requestHeaders(state.Auth.Headers).Get(protocol.AuthHTTPUserAgentName)
		if input.UserAgent == "" {
			input.UserAgent = protocol.AuthUserAgentValue
		}
		return bridgewebsocket.Open(ctx, *input)
	}
	options.Exchange = session.exchange
	options.Validate = session.validate
	return *options
}

func (session *NativeBridgeSession) exchange(ctx context.Context, input bridgeModels.BridgeExchange) error {
	session.mutex.Lock()
	state := cloneNativeAuthState(session.state)
	session.mutex.Unlock()
	state, metadata, err := session.sdk.nativeBridgeExchange(ctx, state, input)
	session.record(state, metadata, err == nil)
	return err
}

func (session *NativeBridgeSession) validate(ctx context.Context, identifier, code string) (bool, error) {
	session.mutex.Lock()
	state := cloneNativeAuthState(session.state)
	session.mutex.Unlock()
	input := new(auth.AuthBridgeCodeRequest)
	input.SessionUUID, input.Code = identifier, code
	state, metadata, status, err := session.sdk.nativeBridgeCode(ctx, state, *input)
	session.record(state, metadata, err == nil)
	return status != http.StatusPreconditionFailed, err
}

func (session *NativeBridgeSession) record(state NativeAuthState, metadata ResponseMetadata, succeeded bool) {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	session.state = cloneNativeAuthState(state)
	if succeeded && metadata.StatusCode != 0 {
		session.responses = append(session.responses, metadata)
	}
}

func (session *NativeBridgeSession) recordResult(result *NativeAuthResult) {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	session.state = cloneNativeAuthState(result.State)
	session.responses = append(session.responses, cloneDriveResponses(result.Responses)...)
}

func (session *NativeBridgeSession) result(success bool) (*NativeAuthResult, error) {
	session.mutex.Lock()
	state, responses := cloneNativeAuthState(session.state), cloneDriveResponses(session.responses)
	session.mutex.Unlock()
	operation := new(nativeAuthOperation)
	operation.name = "NativeBridgeSession.VerifyCode"
	operation.state, operation.responses, operation.success = state, responses, success
	return nativeAuthResult(operation)
}

func (session *NativeBridgeSession) failure(operation string, err error) *ClientError {
	var client *ClientError
	if !errors.As(err, &client) {
		client = newClientError(operation, nativeBridgeErrorKind(err), 0, nil, nil, err)
	}

	current := *client
	client = &current
	client.operation = operation

	session.mutex.Lock()
	prior := cloneDriveResponses(session.responses)
	progress := new(nativeAuthOperation)
	progress.state = cloneNativeAuthState(session.state)

	for _, metadata := range client.prior {
		progress.record(&webtransport.BytesResponse{Status: metadata.StatusCode,
			CookieScopeURL: metadata.CookieScopeURL, Headers: requestHeaders(metadata.Headers), Body: nil})
	}
	if client.status != 0 {
		progress.record(&webtransport.BytesResponse{Status: client.status,
			CookieScopeURL: client.cookieScopeURL, Headers: requestHeaders(client.headers), Body: nil})
	}
	session.state = cloneNativeAuthState(progress.state)
	session.responses = append(session.responses, cloneDriveResponses(progress.responses)...)
	session.mutex.Unlock()

	client.prior = append(prior, client.prior...)
	return client
}

func nativeBridgeErrorKind(err error) ErrorKind {
	switch {
	case errors.Is(err, bridge.ErrClosed), errors.Is(err, bridgewebsocket.ErrClosed):
		return Closed
	case errors.Is(err, bridge.ErrBusy):
		return Busy
	case errors.Is(err, context.Canceled):
		return Canceled
	case errors.Is(err, context.DeadlineExceeded):
		return Timeout
	default:
		return Transport
	}
}

func (session *NativeBridgeSession) bridgeFallbackNotice(err error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}

	session.mutex.Lock()
	defer session.mutex.Unlock()

	challenge := session.state.Challenge
	if len(challenge.PhoneNumbers) == 0 {
		return
	}

	if challenge.Mode != "" && challenge.Mode != string(auth.Sms) ||
		challenge.Mode == "" && challenge.PhoneNumbers[0].PushMode != string(auth.Sms) {
		return
	}

	notice := protocol.AuthBridgeFallbackNoticeValue
	session.state.DeliveryNotice = &notice
}

func bridgeLegacy(push *bridge.Push) bool {
	return push.Payload.Txnid != nil && strings.HasSuffix(*push.Payload.Txnid,
		string(bridgeModels.LegacyTransactionSuffix))
}

func (session *NativeBridgeSession) verifyLegacy(ctx context.Context, state NativeAuthState,
	code string,
) (*NativeAuthResult, error) {
	updated, result, err := session.sdk.nativeBridgeLegacy(ctx, state, code, func() { _ = session.Close() })

	session.mutex.Lock()
	session.state = cloneNativeAuthState(updated)
	session.mutex.Unlock()

	if err != nil {
		return nil, session.failure("NativeBridgeSession.VerifyCode", err)
	}

	session.recordResult(result)
	return session.result(result.Success)
}
