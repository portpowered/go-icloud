package icloud_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

const findMyTestDeadline = 10 * time.Second

type findMySchedulerFunc func(context.Context, icloud.FindMyWaitRequest) error

func (wait findMySchedulerFunc) Wait(ctx context.Context, request icloud.FindMyWaitRequest) error {
	return wait(ctx, request)
}

func findMyAuth() icloud.AuthContext {
	auth := deviceRequest().Auth
	auth.FindMyServiceURL = "https://findmy.example.invalid"
	auth.SetupServiceURL = "https://setup.example.invalid"
	token := "synthetic-session-token"
	auth.SessionToken = &token

	return auth
}

func findMyReply(status int, body string) *http.Response {
	response := new(http.Response)
	response.StatusCode = status
	response.Body = io.NopCloser(strings.NewReader(body))
	response.Header = make(http.Header)
	response.Header.Set("Content-Type", "application/json")

	return response
}

func openFindMy(t *testing.T, transport sdkRoundTrip, options ...icloud.FindMyOption) *icloud.FindMySession {
	t.Helper()

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	session, err := client.OpenFindMySession(t.Context(), icloud.OpenFindMySessionRequest{
		Auth: findMyAuth(), IncludeFamily: false}, options...)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		closeErr := session.Close()
		if closeErr != nil {
			t.Error(closeErr)
		}
	})

	return session
}

func TestFindMyCloseCancelsCurrentAndQueuedCalls(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})

	var calls atomic.Int32

	session := openFindMy(t, func(request *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			return findMyReply(http.StatusOK, `{"content":[{"id":"device"}],"serverContext":{"token":"one"}}`), nil
		}

		close(started)
		<-request.Context().Done()

		return nil, fmt.Errorf("closed Find My peer: %w", request.Context().Err())
	}, icloud.WithFindMyMonitorInterval(0))
	results := make(chan error, driveQueuedCalls+1)

	var workers sync.WaitGroup

	refresh := func() {
		defer workers.Done()

		_, err := session.Refresh(t.Context(), icloud.RefreshFindMyRequest{Locate: true})
		results <- err
	}

	workers.Add(1)

	go refresh()

	<-started

	for range driveQueuedCalls {
		workers.Add(1)

		go refresh()
	}

	for range 2 {
		err := session.Close()
		if err != nil {
			t.Fatal(err)
		}
	}

	workers.Wait()
	close(results)

	checkFindMyClosed(t, session, results, &calls)
}

func TestFindMyMonitorRecoversWithoutLocationOrFamilyPolling(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), findMyTestDeadline)
	defer cancel()

	waiting := make(chan struct{})
	permit := make(chan struct{})
	owned := make(chan *icloud.FindMySession, 1)
	scheduler := findMyControlledScheduler(t, owned, waiting, permit)

	var calls atomic.Int32

	session := openFindMy(t, findMyMonitorPeer(t, &calls),
		icloud.WithFindMyMonitorInterval(time.Minute), icloud.WithFindMyScheduler(scheduler))
	owned <- session

	awaitFindMySignal(ctx, t, waiting)

	permit <- struct{}{}

	awaitFindMySignal(ctx, t, waiting)
	assertSDKKind(t, session.LastError(), icloud.Unavailable)

	permit <- struct{}{}

	awaitFindMySignal(ctx, t, waiting)

	if session.LastError() != nil || calls.Load() != 3 {
		t.Fatal("monitor failed to recover or performed family polling")
	}

	snapshot, err := session.Snapshot()
	if err != nil || snapshot.Devices[0].Name == nil || *snapshot.Devices[0].Name != "updated" {
		t.Fatal("monitor did not publish its successful refresh")
	}

	err = session.Close()
	if err != nil {
		t.Fatal(err)
	}

	awaitFindMySignal(ctx, t, session.MonitorDone())
}

func awaitFindMySignal(ctx context.Context, t *testing.T, signal <-chan struct{}) {
	t.Helper()

	select {
	case <-signal:
	case <-ctx.Done():
		t.Fatal("Find My lifecycle failed to make progress")
	}
}

func TestFindMySchedulerFaultHasNoHTTPReceipt(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), findMyTestDeadline)
	defer cancel()

	scheduler := findMySchedulerFunc(func(_ context.Context, _ icloud.FindMyWaitRequest) error {
		return errSyntheticPeer
	})
	session := openFindMy(t, func(_ *http.Request) (*http.Response, error) {
		return findMyReply(http.StatusOK, `{"content":[{"id":"device"}]}`), nil
	}, icloud.WithFindMyScheduler(scheduler))
	awaitFindMySignal(ctx, t, session.MonitorDone())
	failure := session.LastError()
	assertSDKKind(t, failure, icloud.Transport)

	if !errors.Is(failure, errSyntheticPeer) || len(failure.PriorResponses()) != 0 || len(session.LastResponses()) != 0 {
		t.Fatal("scheduler fault lost its cause or borrowed another operation's HTTP evidence")
	}
}

func checkFindMyClosed(t *testing.T, session *icloud.FindMySession, results <-chan error, calls *atomic.Int32) {
	t.Helper()

	for err := range results {
		var failure *icloud.ClientError
		if !errors.As(err, &failure) || (failure.Kind() != icloud.Canceled && failure.Kind() != icloud.Closed) {
			t.Fatalf("Find My close lost cancellation: %v", err)
		}
	}

	if calls.Load() != 2 || len(session.LastResponses()) != 0 {
		t.Fatal("queued calls reached the transport or invented response evidence")
	}

	snapshot, err := session.Snapshot()
	if err != nil || len(snapshot.Devices) != 1 {
		t.Fatal("close discarded the last successful snapshot")
	}
}

func findMyControlledScheduler(t *testing.T, owned <-chan *icloud.FindMySession,
	waiting chan<- struct{}, permit <-chan struct{},
) findMySchedulerFunc {
	t.Helper()

	var current *icloud.FindMySession

	scheduler := findMySchedulerFunc(func(call context.Context, wait icloud.FindMyWaitRequest) error {
		if current == nil {
			current = <-owned
		}

		_, snapshotErr := current.Snapshot()
		if snapshotErr != nil {
			return fmt.Errorf("snapshot during scheduler callback: %w", snapshotErr)
		}

		if wait.Kind != icloud.FindMyMonitorWait || wait.Delay != time.Minute {
			t.Error("monitor requested the wrong wait")
		}

		select {
		case waiting <- struct{}{}:
		case <-call.Done():
			return fmt.Errorf("monitor canceled before wait: %w", call.Err())
		}

		select {
		case <-permit:
			return nil
		case <-call.Done():
			return fmt.Errorf("monitor canceled in wait: %w", call.Err())
		}
	})

	return scheduler
}

func findMyMonitorPeer(t *testing.T, calls *atomic.Int32) sdkRoundTrip {
	t.Helper()

	return func(request *http.Request) (*http.Response, error) {
		count := calls.Add(1)
		if count == 1 {
			return findMyReply(http.StatusOK, `{"content":[{"id":"device"}],"serverContext":{"token":"one"}}`), nil
		}

		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, fmt.Errorf("read monitor payload: %w", err)
		}

		if strings.Contains(string(body), "shouldLocate") {
			t.Error("monitor asked for updated locations")
		}

		if count == 2 {
			return findMyReply(http.StatusServiceUnavailable, `{"reason":"synthetic unavailable"}`), nil
		}

		return findMyReply(http.StatusOK, `{"content":[{"id":"device","name":"updated"}],"serverContext":{},`+
			`"userInfo":{"hasMembers":true,"membersInfo":{"member":{"deviceFetchStatus":"LOADING"}}}}`), nil
	}
}
