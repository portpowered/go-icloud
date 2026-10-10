package photosync_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/pkg/photosync"
)

const testOriginalVersion = string(icloud.PhotoOriginal)

const (
	testPhotoFilename     = "photo.jpg"
	testCollisionFilename = "Photo.JPG"
	testSecondFilename    = "second.jpg"
	testSameFilename      = "same.jpg"
	testFirstCursor       = "cursor-1"
	testSecondCursor      = "cursor-2"
)

type source struct {
	assets    []photosync.Asset
	cursor    string
	downloads []string
	deletions []string
	visits    int
	available bool
}

func (provider *source) Cursor(context.Context, icloud.AuthContext, photosync.Options) (*string, error) {
	value := provider.cursor

	return &value, nil
}

func (provider *source) Visit(ctx context.Context, _ icloud.AuthContext, _ photosync.Options,
	visit func(photosync.Asset) (bool, error),
) error {
	provider.visits++
	for _, asset := range provider.assets {
		err := ctx.Err()
		if err != nil {
			return fmt.Errorf("source canceled: %w", err)
		}

		more, err := visit(asset)
		if err != nil {
			return fmt.Errorf("source visitor: %w", err)
		}

		if !more {
			return nil
		}
	}

	return nil
}

func (provider *source) Download(_ context.Context, _ icloud.AuthContext, asset photosync.Asset,
	key string,
) ([]byte, bool, error) {
	provider.downloads = append(provider.downloads, asset.ID+":"+key)

	return []byte(asset.ID + ":" + key), provider.available, nil
}

func (provider *source) Delete(_ context.Context, _ icloud.AuthContext, asset photosync.Asset) (bool, error) {
	provider.deletions = append(provider.deletions, asset.ID)

	return true, nil
}

func newSource(assets ...photosync.Asset) *source {
	return &source{assets: assets, cursor: testFirstCursor, downloads: []string{}, deletions: []string{},
		visits: 0, available: true}
}

func photo(assetID, filename string) photosync.Asset {
	taken := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	url := "https://photos.example.invalid/" + assetID
	size := int64(len(assetID + ":original"))
	checksum := "checksum-" + assetID

	return photosync.Asset{ID: assetID, Album: nil, Library: nil, Filename: filename, ItemType: photosync.Image,
		IsLivePhoto: false,
		AddedAt:     &taken, TakenAt: &taken, Metadata: nil, Resources: map[string]photosync.Resource{
			testOriginalVersion: {Key: testOriginalVersion, Filename: filename, Type: nil, Url: &url,
				Size: &size, Checksum: &checksum},
		}}
}

func newEngine(t *testing.T, provider photosync.Source) *photosync.Engine {
	t.Helper()

	client, err := photosync.New(provider, photosync.Configuration{Files: photosync.OSFileSystem{},
		Now: func() time.Time { return time.Date(2026, time.April, 10, 0, 0, 0, 0, time.UTC) }, Wait: nil})
	if err != nil {
		t.Fatal(err)
	}

	return client
}

func request(directory string) photosync.Request {
	auth := new(icloud.AuthContext)
	auth.AccountID, auth.ClientID, auth.Headers = "account-1", "client-1", []icloud.Header{}

	return photosync.Request{Auth: *auth,
		Options: photosync.DefaultOptions(directory)}
}

func TestPersistedManifestAndCursor(t *testing.T) {
	t.Parallel()

	provider := newSource(photo("asset-1", testPhotoFilename))
	client := newEngine(t, provider)
	input := request(filepath.Join(t.TempDir(), "output"))

	first, err := client.Run(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}

	if first.DownloadedCount != 1 {
		t.Fatalf("first result: %+v", first)
	}

	data, err := readPhotoSyncTestFile(t, filepath.Join(input.Options.Directory, testPhotoFilename))
	if err != nil || string(data) != "asset-1:original" {
		t.Fatalf("materialized bytes %q: %v", data, err)
	}

	checkRestartAndSize(t, provider, client, input)
}
func checkRestartAndSize(t *testing.T, provider *source, client *photosync.Engine, input photosync.Request) {
	t.Helper()

	second, err := newEngine(t, provider).Run(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}

	if !second.ShortCircuited || len(provider.downloads) != 1 {
		t.Fatalf("restart did not reuse manifest: %+v", second)
	}

	err = os.WriteFile(filepath.Join(input.Options.Directory, testPhotoFilename), []byte("truncated"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	third, err := client.Run(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}

	if third.ShortCircuited || third.DownloadedCount != 1 {
		t.Fatalf("wrong-sized content trusted: %+v", third)
	}
}

func TestPreviewNeverWritesOrDeletes(t *testing.T) {
	t.Parallel()

	for _, printOnly := range []bool{false, true} {
		provider := newSource(photo("asset-1", testPhotoFilename))
		input := request(filepath.Join(t.TempDir(), "output"))
		input.Options.DryRun, input.Options.OnlyPrintFilenames = !printOnly, printOnly
		input.Options.AutoDelete = true
		days := 0
		input.Options.KeepIcloudRecentDays = &days

		result, err := newEngine(t, provider).Run(t.Context(), input)
		if err != nil {
			t.Fatal(err)
		}

		if result.ListedCount != 1 || len(provider.downloads) != 0 || len(provider.deletions) != 0 {
			t.Fatalf("preview mutated: %+v", result)
		}

		_, err = os.Stat(input.Options.Directory)
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("preview wrote directory: %v", err)
		}
	}
}

func TestStaleCleanupAndRetention(t *testing.T) {
	t.Parallel()

	provider := newSource(photo("asset-1", "first.jpg"), photo("asset-2", testSecondFilename))
	input := request(t.TempDir())

	client := newEngine(t, provider)

	_, err := client.Run(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}

	provider.assets, provider.cursor = provider.assets[:1], testSecondCursor
	input.Options.AutoDelete = true
	days := 5
	input.Options.KeepIcloudRecentDays = &days

	result, err := client.Run(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}

	if result.DeletedCount != 2 || len(provider.deletions) != 1 {
		t.Fatalf("cleanup result: %+v", result)
	}

	_, err = os.Stat(filepath.Join(input.Options.Directory, testSecondFilename))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale file remains: %v", err)
	}
}

func TestMissingDownloadDoesNotAdvanceCursorOrDeleteRemote(t *testing.T) {
	t.Parallel()

	provider := newSource(photo("asset-1", testPhotoFilename))
	provider.available = false
	input := request(t.TempDir())
	days := 0
	input.Options.KeepIcloudRecentDays = &days

	result, err := newEngine(t, provider).Run(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}

	if result.SkippedCount != 1 || len(provider.deletions) != 0 {
		t.Fatalf("missing resource considered safe: %+v", result)
	}

	_, err = os.Stat(result.StatePath)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("incomplete cursor persisted: %v", err)
	}
}

func TestCanceledRunAndWatch(t *testing.T) {
	t.Parallel()

	provider := newSource(photo("asset-1", testPhotoFilename))
	client := newEngine(t, provider)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := client.Run(ctx, request(t.TempDir()))
	if !errors.Is(err, context.Canceled) || provider.visits != 0 {
		t.Fatalf("canceled run: %v", err)
	}

	iterations := 2

	ctx, cancel = context.WithCancel(t.Context())
	defer cancel()

	err = client.Watch(ctx, request(t.TempDir()), photosync.WatchOptions{IntervalSeconds: 1, Iterations: &iterations},
		func(photosync.Result) error {
			cancel()

			return nil
		})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("watch ignored cancellation: %v", err)
	}
}
