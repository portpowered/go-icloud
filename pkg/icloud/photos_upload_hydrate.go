package icloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/photosupload"
)

const photoHydrationTimeout = 60 * time.Second
const photoHydrationInterval = 2 * time.Second
const photoHydrationMaximumInterval = 8 * time.Second
const photoHydrationBackoffFactor = 2

// WithPhotoUploadClock supplies elapsed time for indexing deadlines independently of file timestamps.
// The clock must advance monotonically and be safe for concurrent calls.
func WithPhotoUploadClock(clock func() time.Time) Option {
	return func(config *configuration) error {
		if clock == nil {
			return errNilOption
		}

		config.photoUploadClock = clock

		return nil
	}
}

// WithPhotoUploadWaiter supplies an interruptible retry waiter for indexing.
// The waiter must honor cancellation; production callers normally use the default timer.
func WithPhotoUploadWaiter(waiter func(context.Context, time.Duration) error) Option {
	return func(config *configuration) error {
		if waiter == nil {
			return errNilOption
		}

		config.photoUploadWait = waiter

		return nil
	}
}

func waitPhotoUpload(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(max(delay, 0))
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("wait for photo indexing: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

func (read *photosRead) hydratePhotoUpload(ctx context.Context, request UploadPhotoRequest,
	registration photosupload.PhotosPutAssetResult,
) ([]Photo, error) {
	master, _ := registration.CplMaster.Get()

	asset, _ := registration.CplAsset.Get()
	if master == "" || asset == "" {
		return []Photo{}, nil
	}

	timeout, delay := photoUploadHydrationTiming(request)

	deadline := read.sdk.photoUploadClock().Add(timeout)

	for {
		photo, err := read.lookupUploadedPhoto(ctx, master, asset)
		if len(photo) != 0 {
			return photo, nil
		}

		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || photoUploadSessionFailure(err) {
			return nil, err
		}

		waitErr := read.waitForPhotoIndexing(ctx, deadline, delay, err)
		if errors.Is(waitErr, errPhotoIndexingDeadline) {
			return []Photo{}, photoHydrationFinalError(err)
		}

		if waitErr != nil {
			return nil, waitErr
		}

		delay = min(delay*photoHydrationBackoffFactor, photoHydrationMaximumInterval)
	}
}

func photoHydrationFinalError(err error) error {
	var failure *ClientError
	if errors.As(err, &failure) && failure.Kind() != RateLimited {
		return err
	}

	return nil
}

var errPhotoIndexingDeadline = errors.New("photo indexing deadline expired")

func (read *photosRead) waitForPhotoIndexing(ctx context.Context, deadline time.Time,
	delay time.Duration, lookupErr error,
) error {
	remaining := deadline.Sub(read.sdk.photoUploadClock())
	if remaining <= 0 {
		return errPhotoIndexingDeadline
	}

	var failure *ClientError
	if errors.As(lookupErr, &failure) && failure.Kind() == RateLimited {
		for _, header := range failure.ResponseHeaders() {
			if http.CanonicalHeaderKey(header.Name) == protocol.PhotosUploadHTTPRetryAfterName {
				seconds, err := strconv.ParseFloat(header.Value, 64)
				if err == nil && seconds > 0 {
					delay = time.Duration(min(seconds, remaining.Seconds()) * float64(time.Second))
				}
			}
		}
	}

	err := read.sdk.photoUploadWait(ctx, min(delay, remaining))
	if err != nil {
		kind := Canceled
		if errors.Is(err, context.DeadlineExceeded) {
			kind = Timeout
		}

		return read.failure(err, kind)
	}

	return nil
}

func (read *photosRead) lookupUploadedPhoto(ctx context.Context, masterName, assetName string) ([]Photo, error) {
	response, err := read.sdk.web.PhotosHydrateUpload(ctx, read.auth, masterName, assetName)
	if err != nil {
		failure := read.failure(err, InvalidResponse)
		if failure.StatusCode() != 0 {
			read.responses = append(read.responses, &webtransport.BytesResponse{Status: failure.StatusCode(),
				Body: failure.ResponseBody(), CookieScopeURL: failure.CookieScopeURL(),

				Headers: requestHeaders(failure.ResponseHeaders())})
		}

		return nil, failure
	}

	read.responses = append(read.responses, response.Metadata)

	photos, err := uploadedPhotoRecords(response.Data)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	return photos, nil
}

func uploadedPhotoRecords(data cloudkit.CKLookupResponse) ([]Photo, error) {
	var master, asset *cloudkit.CKRecord

	for _, item := range data.Records {
		decoded, decodeErr := webtransport.DecodeReminderLookupRecord(item)
		if decodeErr != nil {
			return nil, fmt.Errorf("decode uploaded photo: %w", decodeErr)
		}

		if decoded.Record == nil {
			continue
		}

		switch decoded.Record.RecordType {
		case string(cloudkit.CPLMaster):
			master = decoded.Record
		case string(cloudkit.CPLAsset):
			asset = decoded.Record
		}
	}

	if master == nil || asset == nil {
		return []Photo{}, nil
	}

	photo, err := projectPhoto(*master, *asset)
	if err != nil {
		return nil, fmt.Errorf("project uploaded photo: %w", err)
	}

	return []Photo{photo}, nil
}

func photoUploadHydrationTiming(request UploadPhotoRequest) (time.Duration, time.Duration) {
	timeout, delay := photoHydrationTimeout, photoHydrationInterval
	if request.HydrationTimeout != nil {
		timeout = max(*request.HydrationTimeout, 0)
	}

	if request.HydrationInterval != nil {
		delay = max(*request.HydrationInterval, 0)
	}

	return timeout, delay
}

func photoUploadSessionFailure(err error) bool {
	var failure *ClientError
	if !errors.As(err, &failure) {
		return false
	}

	var envelope photosupload.PhotosSessionErrorResponse
	if json.Unmarshal(failure.ResponseBody(), &envelope) != nil {
		return false
	}

	reasons := []*cloudkit.CKUnknownJSON{envelope.ErrorMessage, envelope.Reason,
		envelope.ErrorReason, envelope.Error}
	for _, value := range reasons {
		if value == nil {
			continue
		}

		present, truthErr := reminderTruthy(*value)
		if truthErr == nil && present {
			return true
		}
	}

	return false
}
