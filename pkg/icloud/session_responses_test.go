package icloud_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func TestApplySessionResponsesContextFailure(t *testing.T) {
	t.Parallel()

	for kind, cause := range map[icloud.ErrorKind]error{
		icloud.Canceled: context.Canceled, icloud.Timeout: context.DeadlineExceeded,
	} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()

			client, err := icloud.New(icloud.WithHTTPTransport(sdkRoundTrip(func(_ *http.Request) (*http.Response, error) {
				t.Fatal("response application attempted network access")

				return nil, errSyntheticPeer
			})))
			if err != nil {
				t.Fatal(err)
			}

			request := new(icloud.ApplySessionResponsesRequest)
			request.Session.Auth = deviceRequest().Auth
			request.Session.AccountData = json.RawMessage(`{"synthetic":true}`)

			before, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}

			ctx := failedSessionResponseContext(t, kind)

			result, err := client.ApplySessionResponses(ctx, *request)

			var failure *icloud.ClientError

			if result != nil || !errors.As(err, &failure) || failure.Kind() != kind || !errors.Is(err, cause) {
				t.Fatal("context failure lost its classification or cause")
			}

			after, err := json.Marshal(request)
			if err != nil || string(before) != string(after) {
				t.Fatal("context failure mutated caller-owned state", err)
			}
		})
	}
}

func failedSessionResponseContext(t *testing.T, kind icloud.ErrorKind) context.Context {
	t.Helper()

	if kind == icloud.Timeout {
		ctx, cancel := context.WithDeadline(t.Context(), time.Unix(0, 0))
		cancel()

		return ctx
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	return ctx
}
