package icloud_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

const driveQueuedCalls = 16

func TestDriveSessionCloseCancelsCurrentAndQueuedRequests(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})

	var calls atomic.Int32

	client, err := icloud.New(icloud.WithHTTPTransport(sdkRoundTrip(func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		close(started)
		<-request.Context().Done()

		return nil, fmt.Errorf("closed synthetic peer: %w", request.Context().Err())
	})))
	if err != nil {
		t.Fatal(err)
	}

	session, err := client.OpenDriveSession(t.Context(), icloud.OpenDriveSessionRequest{Auth: driveAuth()})
	if err != nil {
		t.Fatal(err)
	}

	results := make(chan error, driveQueuedCalls+1)

	var workers sync.WaitGroup

	workers.Go(func() {
		_, callErr := session.Root(t.Context(), icloud.DriveLocationRequest{Refresh: false})
		results <- callErr
	})

	<-started

	for range driveQueuedCalls {
		workers.Go(func() {
			_, callErr := session.Root(t.Context(), icloud.DriveLocationRequest{Refresh: false})
			results <- callErr
		})
	}

	for range 2 {
		closeErr := session.Close()
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}

	workers.Wait()
	close(results)

	checkDriveCloseResults(t, session, results, &calls)
}

func TestDriveSessionExpiredCallAndOriginScope(t *testing.T) {
	t.Parallel()

	client, err := icloud.New(icloud.WithHTTPTransport(sdkRoundTrip(func(_ *http.Request) (*http.Response, error) {
		t.Error("local refusal reached the HTTP transport")

		return nil, errSyntheticPeer
	})))
	if err != nil {
		t.Fatal(err)
	}

	auth := driveAuth()
	auth.DriveServiceURL += "/api"
	_, err = client.OpenDriveSession(t.Context(), icloud.OpenDriveSessionRequest{Auth: auth})
	assertSDKKind(t, err, icloud.Configuration)

	session, err := client.OpenDriveSession(t.Context(), icloud.OpenDriveSessionRequest{Auth: driveAuth()})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		closeErr := session.Close()
		if closeErr != nil {
			t.Error(closeErr)
		}
	})

	ctx, cancel := context.WithDeadline(t.Context(), time.Unix(1, 0))
	defer cancel()

	_, err = session.Root(ctx, icloud.DriveLocationRequest{Refresh: false})
	assertSDKKind(t, err, icloud.Timeout)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("expired operation lost its cause")
	}
}

func assertSDKKind(t *testing.T, err error, kind icloud.ErrorKind) {
	t.Helper()

	var failure *icloud.ClientError
	if !errors.As(err, &failure) || failure.Kind() != kind {
		t.Fatalf("expected %s failure: %v", kind, err)
	}
}

func checkDriveCloseResults(t *testing.T, session *icloud.DriveSession, results <-chan error, calls *atomic.Int32) {
	t.Helper()

	for callErr := range results {
		var failure *icloud.ClientError
		if !errors.As(callErr, &failure) || (failure.Kind() != icloud.Closed && failure.Kind() != icloud.Canceled) {
			t.Fatalf("close lost typed cancellation: %v", callErr)
		}
	}

	if calls.Load() != 1 || len(session.LastResponses()) != 0 {
		t.Fatal("queued request reached transport or fabricated response evidence")
	}
}
