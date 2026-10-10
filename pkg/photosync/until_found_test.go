package photosync_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/pkg/photosync"
)

type untilFoundSource struct {
	*source
	trace []string
}

func (provider *untilFoundSource) Cursor(ctx context.Context, auth icloud.AuthContext, options photosync.Options) (*string, error) {
	provider.trace = append(provider.trace, "cursor:"+provider.cursor)

	return provider.source.Cursor(ctx, auth, options)
}

func (provider *untilFoundSource) Visit(ctx context.Context, auth icloud.AuthContext, options photosync.Options,
	visit func(photosync.Asset) (bool, error),
) error {
	return provider.source.Visit(ctx, auth, options, func(asset photosync.Asset) (bool, error) {
		provider.trace = append(provider.trace, "visit:"+asset.ID)

		return visit(asset)
	})
}

func TestUntilFoundPreservesCursorForUnrestrictedRestart(t *testing.T) {
	t.Parallel()

	provider := &untilFoundSource{source: newSource(photo("current", "current.jpg")), trace: []string{}}
	input := request(t.TempDir())
	first, err := newEngine(t, provider).Run(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}

	provider.assets = []photosync.Asset{photo("newest", "newest.jpg"), provider.assets[0], photo("unvisited", "unvisited.jpg")}
	provider.cursor = "cursor-2"
	limit := 1
	input.Options.UntilFound = &limit
	limited, err := newEngine(t, provider).Run(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}

	if limited.StatePath != first.StatePath || limited.ShortCircuited || limited.DownloadedCount != 1 || limited.SkippedCount != 1 {
		t.Fatalf("limited scan did not stop after the current asset: %+v", limited)
	}

	checkUntilFoundManifest(t, first.StatePath, "cursor-1", 2)
	input.Options.UntilFound = nil
	resumed, err := newEngine(t, provider).Run(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}

	if resumed.StatePath != first.StatePath || resumed.ShortCircuited || resumed.DownloadedCount != 1 || resumed.SkippedCount != 2 {
		t.Fatalf("unrestricted restart omitted remaining assets: %+v", resumed)
	}

	checkUntilFoundManifest(t, first.StatePath, "cursor-2", 3)
	last, err := newEngine(t, provider).Run(t.Context(), input)
	if err != nil || !last.ShortCircuited {
		t.Fatalf("complete scan did not enable restart shortcut: %+v, %v", last, err)
	}

	wantTrace := []string{"cursor:cursor-1", "visit:current", "cursor:cursor-2", "visit:newest", "visit:current",
		"cursor:cursor-2", "visit:newest", "visit:current", "visit:unvisited", "cursor:cursor-2"}
	if !reflect.DeepEqual(provider.trace, wantTrace) || !reflect.DeepEqual(provider.downloads,
		[]string{"current:original", "newest:original", "unvisited:original"}) {
		t.Fatalf("unexpected cursor, visit or download consumption: %v, %v", provider.trace, provider.downloads)
	}
}

func checkUntilFoundManifest(t *testing.T, path, cursor string, count int) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var manifest photosync.Manifest
	err = json.Unmarshal(data, &manifest)
	if err != nil {
		t.Fatal(err)
	}

	if manifest.Cursor == nil || *manifest.Cursor != cursor || len(manifest.Resources) != count {
		t.Fatalf("incorrect persisted scan checkpoint: %+v", manifest)
	}
}
