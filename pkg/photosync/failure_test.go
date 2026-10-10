package photosync_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/photosync"
)

type failingPhotoFiles struct{ photosync.OSFileSystem }

func (failingPhotoFiles) WriteAtomic(_ context.Context, _ string, _ []byte) error {
	return fs.ErrPermission
}

func TestPhotoSyncFailedWriteCannotPersistOrDelete(t *testing.T) {
	t.Parallel()

	provider := newSource(photo("asset-1", testPhotoFilename))
	input := request(t.TempDir())
	days := 0
	input.Options.KeepIcloudRecentDays = &days

	files := failingPhotoFiles{OSFileSystem: photosync.OSFileSystem{}}
	engine, err := photosync.New(provider, photosync.Configuration{Files: files, Now: nil, Wait: nil})
	if err != nil {
		t.Fatal(err)
	}

	result, err := engine.Run(t.Context(), input)
	if !errors.Is(err, fs.ErrPermission) || result.DownloadedCount != 0 || len(provider.deletions) != 0 {
		t.Fatalf("failed write was considered durable: %+v %v", result, err)
	}

	_, err = os.Stat(result.StatePath)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("failed write persisted manifest: %v", err)
	}
}

type escapingSidecarFiles struct {
	photosync.OSFileSystem

	outside string
}

func (files escapingSidecarFiles) Resolve(ctx context.Context, path string) (string, error) {
	if strings.HasSuffix(path, ".xmp") {
		return files.outside, nil
	}

	resolved, err := files.OSFileSystem.Resolve(ctx, path)
	if err != nil {
		return "", fmt.Errorf("resolve test path: %w", err)
	}

	return resolved, nil
}

func TestPhotoSyncSidecarCannotFollowOutsideDestination(t *testing.T) {
	t.Parallel()

	asset := photo("asset-1", testPhotoFilename)
	asset.Metadata = new(photosync.Metadata)

	provider := newSource(asset)
	input := request(filepath.Join(t.TempDir(), "photos"))
	input.Options.XmpSidecar = true

	outside := filepath.Join(t.TempDir(), "outside.xmp")
	files := escapingSidecarFiles{OSFileSystem: photosync.OSFileSystem{}, outside: outside}
	engine, err := photosync.New(provider, photosync.Configuration{Files: files, Now: nil, Wait: nil})
	if err != nil {
		t.Fatal(err)
	}

	result, err := engine.Run(t.Context(), input)

	var failure *photosync.SyncError

	if !errors.As(err, &failure) || result.DownloadedCount != 0 {
		t.Fatalf("escaped sidecar considered durable: %+v %v", result, err)
	}

	_, err = os.Stat(outside)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("outside sidecar was written: %v", err)
	}
}

func TestPhotoSyncBoundedWatchDoesNotWaitAfterFinalRun(t *testing.T) {
	t.Parallel()

	provider := newSource(photo("asset-1", testPhotoFilename))
	waits := 0
	engine, err := photosync.New(provider, photosync.Configuration{Files: photosync.OSFileSystem{}, Now: nil,
		Wait: func(context.Context, time.Duration) error {
			waits++

			return nil
		}})
	if err != nil {
		t.Fatal(err)
	}

	iterations, yields := 2, 0
	err = engine.Watch(t.Context(), request(t.TempDir()),
		photosync.WatchOptions{IntervalSeconds: 1, Iterations: &iterations},
		func(photosync.Result) error {
			yields++

			return nil
		})
	if err != nil || waits != 1 || yields != 2 {
		t.Fatalf("bounded watch: waits=%d yields=%d error=%v", waits, yields, err)
	}
}

func TestPhotoSyncRetentionDoesNotOverflowLargeDayIntervals(t *testing.T) {
	t.Parallel()

	provider := newSource(photo("asset-1", testPhotoFilename))
	input := request(t.TempDir())
	days := 999999999
	input.Options.KeepIcloudRecentDays = &days
	result, err := newEngine(t, provider).Run(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}

	if result.DeletedCount != 0 || len(provider.deletions) != 0 {
		t.Fatalf("large retention interval overflowed: %+v", result)
	}
}

func TestPhotoSyncRecentCutoffRejectsPythonDateOverflow(t *testing.T) {
	t.Parallel()

	provider := newSource(photo("asset-1", testPhotoFilename))
	input := request(t.TempDir())
	days := 999999999
	input.Options.Recent = &days
	result, err := newEngine(t, provider).Run(t.Context(), input)

	var failure *photosync.SyncError
	if !errors.As(err, &failure) || result == nil || provider.visits != 0 {
		t.Fatalf("unrepresentable Python cutoff continued enumeration: result=%+v error=%v", result, err)
	}
}

func TestPhotoSyncCollisionUsesSourceUnicodeCodePoints(t *testing.T) {
	t.Parallel()

	provider := newSource(photo("first", testSameFilename), photo("photo-\u76f8\u518c\u7f16\u53f7αβγδε", testSameFilename))
	input := request(t.TempDir())
	input.Options.OnlyPrintFilenames = true
	result, err := newEngine(t, provider).Run(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}

	// Pinned Source _unique_relative_path uses Python's first eight Unicode code points.
	if len(result.Items) != 2 || result.Items[1].Path != "same_photo-\u76f8\u518c.jpg" {
		t.Fatalf("Unicode Source discriminator differs: %+v", result.Items)
	}
}
