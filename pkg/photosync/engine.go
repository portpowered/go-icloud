package photosync

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

var (
	errConfiguration = errors.New("photo source and filesystem must be nonnil")
	errBusy          = errors.New("photo sync engine already has an active run")
	errOptions       = errors.New("invalid photo sync options")
)

const (
	runOperation   = "Run"
	watchOperation = "Watch"
)

// Source supplies account-scoped photo operations. Visit stops when the visitor returns false.
// Implementations must preserve source response evidence in their returned errors.
type Source interface {
	Cursor(ctx context.Context, auth icloud.AuthContext, options Options) (*string, error)
	Visit(ctx context.Context, auth icloud.AuthContext, options Options, visitor func(Asset) (bool, error)) error
	Download(ctx context.Context, auth icloud.AuthContext, asset Asset, key string) ([]byte, bool, error)
	Delete(ctx context.Context, auth icloud.AuthContext, asset Asset) (bool, error)
}

// FileSystem owns local I/O. WriteAtomic must replace a file only after durable successful writing.
type FileSystem interface {
	ReadFile(ctx context.Context, path string) ([]byte, error)
	Size(ctx context.Context, path string) (int64, error)
	WriteAtomic(ctx context.Context, path string, data []byte) error
	Remove(ctx context.Context, path string) error
	Resolve(ctx context.Context, path string) (string, error)
}

// SyncError preserves a local or injected dependency cause, including cancellation.
type SyncError struct {
	Operation string
	Cause     error
}

// Error identifies the operation without disclosing authentication values.
func (failure *SyncError) Error() string {
	return fmt.Sprintf("photosync %s failed", failure.Operation)
}

// Unwrap permits errors.Is and errors.As on the original failure.
func (failure *SyncError) Unwrap() error { return failure.Cause }

// Configuration supplies caller-owned filesystem, UTC clock and cancellable wait dependencies.
type Configuration struct {
	Files FileSystem
	Now   func() time.Time
	Wait  func(context.Context, time.Duration) error
}

// Engine coordinates an injected source and filesystem; it stores no account credentials.
// One engine permits one active run. Watch invokes callbacks synchronously without owned goroutines.
type Engine struct {
	source  Source
	files   FileSystem
	now     func() time.Time
	wait    func(context.Context, time.Duration) error
	running atomic.Bool
}

// New constructs an engine; omitted clock and wait dependencies use time.Now and a context-aware timer.
func New(source Source, config Configuration) (*Engine, error) {
	if source == nil || config.Files == nil {
		return nil, &SyncError{Operation: "New", Cause: errConfiguration}
	}

	if config.Now == nil {
		config.Now = time.Now
	}

	if config.Wait == nil {
		config.Wait = waitContext
	}

	return &Engine{source: source, files: config.Files, now: config.Now, wait: config.Wait, running: atomic.Bool{}}, nil
}

func waitContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("wait for next photo sync: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

// Watch performs bounded or continuous runs. A callback error stops immediately; no next sleep occurs.
func (engine *Engine) Watch(ctx context.Context, request Request, options WatchOptions,
	yield func(Result) error,
) error {
	if invalidWatch(options, yield) {
		return &SyncError{Operation: watchOperation, Cause: errOptions}
	}

	for completed := 0; options.Iterations == nil || completed < *options.Iterations; completed++ {
		result, err := engine.Run(ctx, request)
		if err != nil {
			return err
		}

		err = yield(*result)
		if err != nil {
			return &SyncError{Operation: watchOperation, Cause: err}
		}

		if options.Iterations != nil && completed+1 >= *options.Iterations {
			return nil
		}

		err = engine.wait(ctx, time.Duration(options.IntervalSeconds)*time.Second)
		if err != nil {
			return &SyncError{Operation: watchOperation, Cause: err}
		}
	}

	return nil
}

func invalidWatch(options WatchOptions, yield func(Result) error) bool {
	const maximumIntervalSeconds = int64((1<<63 - 1) / time.Second)

	return options.IntervalSeconds < 1 || int64(options.IntervalSeconds) > maximumIntervalSeconds ||
		options.Iterations != nil && *options.Iterations < 1 || yield == nil
}
