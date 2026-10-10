package icloud_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

type photoUploadEntropyCounter struct{ reads int }

func (counter *photoUploadEntropyCounter) Read([]byte) (int, error) {
	counter.reads++

	return 0, io.EOF
}

func TestPhotoUploadAlreadyCanceledAvoidsContentAndEntropy(t *testing.T) {
	t.Parallel()

	calls := 0
	entropy := new(photoUploadEntropyCounter)

	client, err := icloud.New(icloud.WithRandomSource(entropy),
		icloud.WithHTTPTransport(sdkRoundTrip(func(*http.Request) (*http.Response, error) {
			calls++

			return nil, errSyntheticPeer
		})))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	cases := photoCanceledUploadOperations(client)
	for name, operation := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			failure := operation(ctx)

			var clientError *icloud.ClientError

			if !errors.Is(failure, context.Canceled) || !errors.As(failure, &clientError) ||
				clientError.Kind() != icloud.Canceled {
				t.Fatal("upload cancellation was not typed", failure)
			}
		})
	}

	t.Cleanup(func() {
		if calls != 0 || entropy.reads != 0 {
			t.Fatal("canceled upload reached a network or entropy boundary")
		}
	})
}

//nolint:wrapcheck // These controls inspect the original typed SDK errors without adding a wrapper.
func photoCanceledUploadOperations(client *icloud.SDK) map[string]func(context.Context) error {
	return map[string]func(context.Context) error{
		"reserve": func(ctx context.Context) error {
			request := new(icloud.ReservePhotoUploadsRequest)
			_, err := client.ReservePhotoUploads(ctx, *request)

			return err
		},
		"send": func(ctx context.Context) error {
			request := new(icloud.SendPhotoUploadBytesRequest)
			_, err := client.SendPhotoUploadBytes(ctx, *request)

			return err
		},
		"register": func(ctx context.Context) error {
			request := new(icloud.RegisterPhotoUploadsRequest)
			_, err := client.RegisterPhotoUploads(ctx, *request)

			return err
		},
		"status": func(ctx context.Context) error {
			request := new(icloud.GetPhotoUploadStatusRequest)
			_, err := client.GetPhotoUploadStatus(ctx, *request)

			return err
		},
		"file": func(ctx context.Context) error {
			request := new(icloud.UploadPhotoFileRequest)
			_, err := client.UploadPhotoFile(ctx, *request)
			return err
		},
		"upload": func(ctx context.Context) error {
			request := new(icloud.UploadPhotoRequest)
			_, err := client.UploadPhoto(ctx, *request)

			return err
		},
	}
}

func TestPhotoUploadWaiterRejectsNil(t *testing.T) {
	t.Parallel()

	_, err := icloud.New(icloud.WithPhotoUploadWaiter(nil))

	var failure *icloud.ClientError

	if !errors.As(err, &failure) || failure.Kind() != icloud.Configuration {
		t.Fatal("nil indexing waiter was accepted", err)
	}
}

func TestPhotoUploadClockRejectsNil(t *testing.T) {
	t.Parallel()
	_, err := icloud.New(icloud.WithPhotoUploadClock(nil))

	var failure *icloud.ClientError

	if !errors.As(err, &failure) || failure.Kind() != icloud.Configuration {
		t.Fatal("nil indexing elapsed clock was accepted", err)
	}
}
