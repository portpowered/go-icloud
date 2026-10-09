package replay_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/pkg/photosync"
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
	return &source{assets: assets, cursor: "cursor-1", downloads: []string{}, deletions: []string{},
		visits: 0, available: true}
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
