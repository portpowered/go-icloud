package replay_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/photosync"
)

type dateFormatCase struct {
	Name               string    `json:"name"`
	Instant            time.Time `json:"instant"`
	Format             string    `json:"format"`
	Path               string    `json:"path"`
	Error              bool      `json:"error"`
	RuntimeUnsupported bool      `json:"runtimeUnsupported"`
}

type dateFormatOracle struct {
	Provenance string           `json:"provenance"`
	Cases      []dateFormatCase `json:"cases"`
}

func TestPinnedSourceDateFormatPathsAndErrors(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("fixtures/synthetic/local/photos-dateformat.json")
	if err != nil {
		t.Fatal(err)
	}

	oracle := new(dateFormatOracle)
	err = json.Unmarshal(data, oracle)
	if err != nil {
		t.Fatal(err)
	}

	if oracle.Provenance == "" || len(oracle.Cases) == 0 {
		t.Fatal("Source oracle is missing")
	}

	directory := t.TempDir()
	for _, test := range oracle.Cases {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			runDateFormatCase(t, test, directory)
		})
	}
}

func runDateFormatCase(t *testing.T, test dateFormatCase, directory string) {
	t.Helper()

	asset := dateFormatAsset()
	_, offset := test.Instant.Zone()
	test.Instant = test.Instant.In(time.FixedZone("", offset))
	asset.TakenAt = &test.Instant
	input := request(directory)
	input.Options.OnlyPrintFilenames = true
	input.Options.FolderStructure = test.Format
	result, err := dateFormatEngine(t, asset).Run(t.Context(), input)
	if test.RuntimeUnsupported {
		var unsupported *photosync.UnsupportedDateFormatError
		if !errors.As(err, &unsupported) {
			t.Fatalf("valid runtime-specific Source format was not explicitly rejected: %v", err)
		}

		return
	}

	if test.Error {
		if err == nil {
			t.Fatalf("Source formatting error accepted: %+v", result)
		}

		return
	}

	if err != nil {
		t.Fatalf("Source format failed: %v", err)
	}

	if len(result.Items) != 1 || result.Items[0].Path != test.Path {
		t.Fatalf("Source path %q differs: %+v", test.Path, result.Items)
	}
}

func dateFormatAsset() photosync.Asset {
	asset := new(photosync.Asset)
	asset.ID, asset.Filename, asset.ItemType = "asset-dateformat", "photo.jpg", photosync.Image
	resource := new(photosync.Resource)
	resource.Key, resource.Filename = string(photosync.OptionsSizeOriginal), asset.Filename
	url := "https://photos.example.invalid/dateformat"
	resource.Url = &url
	asset.Resources = map[string]photosync.Resource{resource.Key: *resource}

	return *asset
}

type dateFormatFileSystem struct {
	photosync.OSFileSystem
}

func (dateFormatFileSystem) Resolve(_ context.Context, path string) (string, error) {
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve synthetic date path: %w", err)
	}

	return resolved, nil
}

func dateFormatEngine(t *testing.T, asset photosync.Asset) *photosync.Engine {
	t.Helper()

	configuration := new(photosync.Configuration)
	configuration.Files = dateFormatFileSystem{OSFileSystem: photosync.OSFileSystem{}}
	configuration.Now = func() time.Time { return time.Date(2026, time.April, 10, 0, 0, 0, 0, time.UTC) }
	client, err := photosync.New(newSource(asset), *configuration)
	if err != nil {
		t.Fatal(err)
	}

	return client
}
