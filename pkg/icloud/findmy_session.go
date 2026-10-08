package icloud

import (
	"context"
	"errors"
	"sync"

	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/internal/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/findmy"
)

var (
	errFindMyClosed     = errors.New("find my session is closed")
	errFindMyNoDevices  = errors.New("find my returned no devices")
	errFindMyDevice     = errors.New("find my device was not found")
	errFindMyCapability = errors.New("find my command is unavailable for this device")
	errFindMyEraseToken = errors.New("find my erase token is unavailable")
)

// FindMySession owns copied credentials, device cache and one cancellable monitor.
// Network calls serialize without holding state locks during I/O or scheduler callbacks.
type FindMySession struct {
	client    *SDK
	mu        sync.Mutex
	auth      AuthContext
	config    findMyConfiguration
	family    bool
	devices   map[string]findmy.FindMyDevice
	order     []string
	user      nullable.Nullable[findmy.FindMyUserInfo]
	server    findmy.FindMyRefreshContext
	responses []ResponseMetadata
	lastError *ClientError
	//nolint:containedctx // API-04, GO-09: the explicit session owns cancellation of its lifetime.
	lifetime    context.Context
	cancel      context.CancelFunc
	gate        chan struct{}
	monitorDone chan struct{}
}

// OpenFindMySession discovers devices, performs bounded family polling and starts one monitor.
// The opening context limits the session lifetime; callers should defer Close after success.
func (sdk *SDK) OpenFindMySession(ctx context.Context, request OpenFindMySessionRequest,
	options ...FindMyOption,
) (*FindMySession, error) {
	const operation = "OpenFindMySession"

	config, err := findMyConfig(options)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	boundary, err := findMyRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	err = webtransport.ValidateOrigin(boundary.Origin)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	lifetime, cancel := context.WithCancel(ctx)

	session := &FindMySession{client: sdk, mu: sync.Mutex{}, auth: cloneDriveAuth(request.Auth),
		config: config, family: request.IncludeFamily, devices: make(map[string]findmy.FindMyDevice),
		order: nil, user: nil, server: nil, responses: nil, lastError: nil,
		lifetime: lifetime, cancel: cancel, gate: make(chan struct{}, 1), monitorDone: make(chan struct{})}
	session.gate <- struct{}{}

	session.bindCookies()

	_, err = session.refresh(ctx, operation, true, true)
	if err != nil {
		cancel()

		return nil, err
	}

	if config.monitorInterval > 0 {
		go session.monitor()
	} else {
		close(session.monitorDone)
	}

	return session, nil
}

// Close cancels current requests, queued calls and waits; repeated calls are safe.
// It does not close the caller-owned transport or scheduler, and does not wait on user code.
func (session *FindMySession) Close() error {
	session.cancel()

	return nil
}

// MonitorDone closes when the session's owned monitor exits; it is closed immediately when disabled.
func (session *FindMySession) MonitorDone() <-chan struct{} { return session.monitorDone }

// Authentication returns copied account credentials and received cookie updates.
func (session *FindMySession) Authentication() AuthContext {
	session.mu.Lock()
	defer session.mu.Unlock()

	return cloneDriveAuth(session.auth)
}

// LastResponses returns copied evidence for every exchange in the latest refresh or command.
// Capability refusals retain prior evidence; a monitor scheduler failure clears it since no request occurred.
func (session *FindMySession) LastResponses() []ResponseMetadata {
	session.mu.Lock()
	defer session.mu.Unlock()

	return cloneDriveResponses(session.responses)
}

func findMyRequestContext(auth AuthContext) (webtransport.RequestContext, error) {
	boundary, err := accountRequestContext(auth)
	if err != nil {
		return boundary, err
	}

	boundary.Origin = auth.FindMyServiceURL

	return boundary, nil
}

func (session *FindMySession) begin(ctx context.Context, operation string) (context.Context, func(), error) {
	return beginSessionCall(ctx, session.lifetime, session.gate, operation, errFindMyClosed)
}
