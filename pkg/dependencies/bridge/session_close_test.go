package bridge_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/dependencies/bridge"
	models "github.com/portpowered/go-icloud/pkg/dependencymodels/bridge"
	pb "github.com/portpowered/go-icloud/pkg/dependencymodels/bridgepb"
)

const concurrentCloseObservation = 25 * time.Millisecond

var errBlockedSocketClose = errors.New("synthetic blocked socket close failure")

type blockingCloseSocket struct {
	*sessionSocket

	entered chan struct{}
	release chan struct{}
	once    sync.Once
	calls   atomic.Int32
}

func (socket *blockingCloseSocket) Close() error {
	socket.calls.Add(1)
	socket.once.Do(func() { close(socket.entered) })
	<-socket.release

	return errors.Join(errBlockedSocketClose, socket.sessionSocket.Close())
}

func TestSessionConcurrentCloseWaitsForCancellationCleanup(t *testing.T) {
	t.Parallel()

	socket := new(blockingCloseSocket)
	socket.sessionSocket = newSessionSocket()
	socket.entered, socket.release = make(chan struct{}), make(chan struct{})
	releaseOnce := new(sync.Once)
	unblock := func() { releaseOnce.Do(func() { close(socket.release) }) }

	defer unblock()

	enqueueToken(t, socket.sessionSocket, pb.Status_STATUS_OK, 0)
	enqueuePush(t, socket.sessionSocket, unitSession, models.ProverShareStep, `,"salt":"`+unitSalt+`"`)
	options := unitOptions(t, socket.sessionSocket)
	options.Open = func(_ context.Context, address string) (bridge.Socket, error) {
		verifyBootstrap(t, address)

		return socket, nil
	}

	ctx, cancel := context.WithCancel(t.Context())

	defer cancel()

	session, err := bridge.Start(ctx, unitData(), options)
	if err != nil {
		t.Fatal(err)
	}

	// Cancellation owns the first Close and deliberately blocks inside socket I/O.
	cancel()
	<-socket.entered

	if session.Active() {
		t.Fatal("closing session still exposes an active connection")
	}

	returned := make(chan error, 1)
	go func() { returned <- session.Close() }()

	observation := time.NewTimer(concurrentCloseObservation)

	defer observation.Stop()

	select {
	case early := <-returned:
		unblock()
		t.Fatalf("concurrent Close returned before socket cleanup: %v", early)
	case <-observation.C:
	}

	unblock()

	closeErr := <-returned
	assertBlockedCloseFailure(t, closeErr)
	assertBlockedCloseFailure(t, session.Close())

	if socket.calls.Load() != 1 {
		t.Fatalf("underlying socket closed %d times", socket.calls.Load())
	}

	select {
	case <-socket.closed:
	default:
		t.Fatal("Close returned before the underlying connection was closed")
	}
}

func assertBlockedCloseFailure(t *testing.T, err error) {
	t.Helper()

	var failure *bridge.ProtocolError
	if !errors.As(err, &failure) || failure.Stage != "close" || !errors.Is(err, errBlockedSocketClose) {
		t.Fatalf("typed close failure lost: %v", err)
	}
}
