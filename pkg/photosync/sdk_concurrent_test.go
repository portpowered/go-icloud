package photosync_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/pkg/photosync"
)

type concurrentSessionClient struct {
	*icloud.SDK

	mu    sync.Mutex
	calls int
	ready chan struct{}
}

func (client *concurrentSessionClient) ApplySessionResponses(ctx context.Context,
	input icloud.ApplySessionResponsesRequest,
) (*icloud.ApplySessionResponsesResult, error) {
	if len(input.Responses) > 0 {
		client.mu.Lock()
		client.calls++

		call := client.calls

		if call == 2 {
			close(client.ready)
		}
		client.mu.Unlock()

		if call <= 2 {
			select {
			case <-client.ready:
			case <-ctx.Done():
				return nil, fmt.Errorf("wait for response overlap: %w", ctx.Err())
			}
		}
	}

	result, err := client.SDK.ApplySessionResponses(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("apply test session responses: %w", err)
	}

	return result, nil
}

func (client *concurrentSessionClient) DownloadPhoto(_ context.Context,
	input icloud.DownloadPhotoRequest,
) (*icloud.DownloadPhotoResult, error) {
	result := new(icloud.DownloadPhotoResult)
	result.Content.Set([]byte(input.PhotoID))
	result.Responses = cookieReceipt(input.PhotoID)

	return result, nil
}

func TestSDKSourceConcurrentResponsesAreNotLost(t *testing.T) {
	t.Parallel()

	base, err := icloud.New()
	if err != nil {
		t.Fatal(err)
	}

	client := &concurrentSessionClient{SDK: base, mu: sync.Mutex{}, calls: 0, ready: make(chan struct{})}
	input := request(t.TempDir())
	session := new(icloud.ResumeSessionResult)
	session.Auth, session.Responses = input.Auth, []icloud.ResponseMetadata{}
	session.AccountCountryCode.SetNull()

	source, err := photosync.NewSDKSource(t.Context(), client, *session, nil)
	if err != nil {
		t.Fatal(err)
	}

	failures := make(chan error, 2)

	for _, id := range []string{"first", "second"} {
		go func() {
			_, _, failure := source.Download(t.Context(), input.Auth, photo(id, id+".jpg"), testOriginalVersion)
			failures <- failure
		}()
	}

	for range 2 {
		failure := <-failures
		if failure != nil {
			t.Fatal(failure)
		}
	}

	snapshot, err := source.Snapshot(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if len(snapshot.Responses) != 2 {
		t.Fatalf("concurrent receipt loss: %d", len(snapshot.Responses))
	}
}
