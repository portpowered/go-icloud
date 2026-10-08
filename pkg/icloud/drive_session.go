package icloud

import (
	"context"
	"errors"
	"sync"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/internal/webtransport"
)

var (
	errDriveClosed    = errors.New("drive session is closed")
	errDriveDirectory = errors.New("drive entry is not a directory")
	errDriveTrash     = errors.New("drive entry is not in trash")
	errDriveChild     = errors.New("drive child was not found")
	errDriveItems     = errors.New("drive folder has no item list")
	errDriveFields    = errors.New("drive entry lacks required operation metadata")
)

// DriveSession owns one account's copied credentials, root/trash cache and operations.
// Calls serialize without holding a state lock during I/O; Close cancels in-flight work.
type DriveSession struct {
	client    *SDK
	mu        sync.Mutex
	auth      AuthContext
	responses []ResponseMetadata
	root      *DriveEntry
	trash     *DriveEntry
	//nolint:containedctx // API-04, GO-09: this explicit session owns cancellation of its whole lifetime.
	lifetime context.Context
	cancel   context.CancelFunc
	gate     chan struct{}
}

// OpenDriveSession copies the account context and starts a cancellable cache lifecycle.
// It performs no network request. The opening context limits the session's lifetime.
func (sdk *SDK) OpenDriveSession(ctx context.Context, request OpenDriveSessionRequest) (*DriveSession, error) {
	boundary, err := driveRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError("OpenDriveSession", Configuration, 0, nil, nil, err)
	}

	err = webtransport.ValidateOrigin(boundary.Origin)
	if err != nil {
		return nil, newClientError("OpenDriveSession", Configuration, 0, nil, nil, err)
	}

	auth := cloneDriveAuth(request.Auth)

	lifetime, cancel := context.WithCancel(ctx)

	session := &DriveSession{client: sdk, mu: sync.Mutex{}, auth: auth, responses: nil, root: nil, trash: nil,
		lifetime: lifetime, cancel: cancel, gate: make(chan struct{}, 1)}
	session.gate <- struct{}{}

	session.bindInitialCookies()

	return session, nil
}

// Close cancels current and queued work and is safe to call repeatedly.
// It does not close the shared client, transport or caller-owned readers.
func (session *DriveSession) Close() error {
	session.cancel()

	return nil
}

// Authentication returns a copied credential snapshot, including upload-token/cookie updates.
// The caller owns storage of these secret values; the reusable client never retains them.
func (session *DriveSession) Authentication() AuthContext {
	session.mu.Lock()
	defer session.mu.Unlock()

	return cloneDriveAuth(session.auth)
}

// LastResponses returns copied response evidence from the latest network operation.
// Cached/local calls leave this evidence unchanged; a failed operation includes completed prior stages.
func (session *DriveSession) LastResponses() []ResponseMetadata {
	session.mu.Lock()
	defer session.mu.Unlock()

	return cloneDriveResponses(session.responses)
}

func cloneDriveResponses(values []ResponseMetadata) []ResponseMetadata {
	result := append([]ResponseMetadata(nil), values...)
	for index := range result {
		result[index].Headers = append([]Header(nil), result[index].Headers...)
	}

	return result
}

// Root returns the cached root, or replaces it after a requested fresh read.
func (session *DriveSession) Root(ctx context.Context, request DriveLocationRequest) (*DriveEntry, error) {
	return session.location(ctx, request.Refresh, false)
}

// Trash returns the cached trash, or replaces it after a requested fresh read.
func (session *DriveSession) Trash(ctx context.Context, request DriveLocationRequest) (*DriveEntry, error) {
	return session.location(ctx, request.Refresh, true)
}

// Lookup selects an exact child name beneath the session root.
func (session *DriveSession) Lookup(ctx context.Context, request DriveLookupRequest) (*DriveEntry, error) {
	root, err := session.Root(ctx, DriveLocationRequest{Refresh: false})
	if err != nil {
		return nil, err
	}

	return root.Lookup(ctx, request)
}

// Directory returns the ordered root child display names.
func (session *DriveSession) Directory(ctx context.Context, request DriveEntryRequest) (*DriveDirectoryResult, error) {
	root, err := session.Root(ctx, DriveLocationRequest{Refresh: false})
	if err != nil {
		return nil, err
	}

	return root.Directory(ctx, request)
}

func (session *DriveSession) begin(ctx context.Context, operation string) (context.Context, func(), error) {
	select {
	case <-session.lifetime.Done():
		return nil, nil, newClientError(operation, Closed, 0, nil, nil, errDriveClosed)
	default:
	}

	select {
	case <-ctx.Done():
		return nil, nil, driveContextFailure(operation, ctx.Err())
	case <-session.lifetime.Done():
		return nil, nil, newClientError(operation, Closed, 0, nil, nil, errDriveClosed)
	case <-session.gate:
	}

	if session.lifetime.Err() != nil {
		session.gate <- struct{}{}

		return nil, nil, newClientError(operation, Closed, 0, nil, nil, errDriveClosed)
	}

	if ctx.Err() != nil {
		session.gate <- struct{}{}

		return nil, nil, driveContextFailure(operation, ctx.Err())
	}

	call, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(session.lifetime, cancel)
	finish := func() {
		stop()
		cancel()

		session.gate <- struct{}{}
	}

	return call, finish, nil
}

func driveContextFailure(operation string, err error) *ClientError {
	kind := Canceled
	if errors.Is(err, context.DeadlineExceeded) {
		kind = Timeout
	}

	return newClientError(operation, kind, 0, nil, nil, err)
}

func (session *DriveSession) location(ctx context.Context, refresh, trash bool) (*DriveEntry, error) {
	call, finish, err := session.begin(ctx, "DriveLocation")
	if err != nil {
		return nil, err
	}
	defer finish()

	session.mu.Lock()
	entry := session.root

	identifier := protocol.DriveRootIdentifierValue

	if trash {
		entry, identifier = session.trash, protocol.DriveTrashIdentifierValue
	}
	session.mu.Unlock()

	if entry != nil && !refresh {
		return entry, nil
	}

	result, err := session.client.GetDriveNode(call, GetDriveNodeRequest{Auth: session.Authentication(),
		NodeID: identifier, ShareID: nil})
	if err != nil {
		session.observeFailure(err)

		return nil, err
	}

	session.observe(result.Metadata)

	entry = &DriveEntry{session: session, data: result.Node, children: nil}
	session.mu.Lock()
	if trash {
		session.trash = entry
	} else {
		session.root = entry
	}
	session.mu.Unlock()

	return entry, nil
}
