package photosync_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/pkg/photosync"
)

func TestPhotoSyncPortableFilenameCollisions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		first  string
		second string
	}{
		{name: "ASCII", first: testCollisionFilename, second: testPhotoFilename},
		{name: "accent", first: "\u00c4.JPG", second: "\u00e4.jpg"},
		{name: "sigma", first: "\u03a3.JPG", second: "\u03c2.jpg"},
		{name: "dotless-i", first: "I.JPG", second: "\u0131.jpg"},
		{name: "Kelvin", first: "K.JPG", second: "\u212a.jpg"},
		{name: "trailing-dot", first: testCollisionFilename, second: "Photo.JPG."},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			checkPortableCollision(t, testCase.first, testCase.second)
		})
	}
}

func checkPortableCollision(t *testing.T, firstName, secondName string) {
	t.Helper()

	provider := newSource(photo("asset-1", firstName), photo("asset-2", secondName))
	input := request(filepath.Join(t.TempDir(), "photos"))
	result, err := newEngine(t, provider).Run(t.Context(), input)
	if err != nil || result == nil || result.DownloadedCount != 2 {
		t.Fatalf("initial collision run: %+v %v", result, err)
	}

	manifest := readCollisionManifest(t, result.StatePath)
	checkCollisionBytes(t, input.Options.Directory, manifest, 2)

	restart, err := newEngine(t, provider).Run(t.Context(), input)
	if err != nil || restart == nil || !restart.ShortCircuited || len(provider.downloads) != 2 {
		t.Fatalf("durable restart: %+v %v downloads=%v", restart, err, provider.downloads)
	}

	// A new asset arrives first, exercising tracked reservations before the
	// existing assets have reserved their paths in this run.
	provider.assets = []photosync.Asset{photo("asset-3", secondName),
		photo("asset-2", secondName), photo("asset-1", firstName)}
	provider.cursor = testSecondCursor
	changed, err := newEngine(t, provider).Run(t.Context(), input)
	if err != nil || changed == nil || changed.DownloadedCount != 1 || len(provider.downloads) != 3 {
		t.Fatalf("tracked collision run: %+v %v downloads=%v", changed, err, provider.downloads)
	}

	manifest = readCollisionManifest(t, changed.StatePath)
	checkCollisionBytes(t, input.Options.Directory, manifest, 3)

	restart, err = newEngine(t, provider).Run(t.Context(), input)
	if err != nil || restart == nil || !restart.ShortCircuited || len(provider.downloads) != 3 {
		t.Fatalf("changed cursor restart: %+v %v downloads=%v", restart, err, provider.downloads)
	}
}

func readCollisionManifest(t *testing.T, statePath string) photosync.Manifest {
	t.Helper()

	data, err := readPhotoSyncTestFile(t, filepath.Clean(statePath))
	if err != nil {
		t.Fatal(err)
	}

	var manifest photosync.Manifest

	err = json.Unmarshal(data, &manifest)
	if err != nil {
		t.Fatal(err)
	}

	return manifest
}

func checkCollisionBytes(t *testing.T, directory string, manifest photosync.Manifest, count int) {
	t.Helper()

	if len(manifest.Resources) != count {
		t.Fatalf("manifest resources=%+v", manifest.Resources)
	}

	for _, resource := range manifest.Resources {
		data, err := readPhotoSyncTestFile(t, filepath.Join(directory, filepath.FromSlash(resource.RelativePath)))
		if err != nil || string(data) != resource.AssetID+":"+resource.ResourceKey {
			t.Fatalf("asset %s path %s contains %q: %v", resource.AssetID, resource.RelativePath, data, err)
		}
	}
}

func TestPhotoSyncRejectsAmbiguousPersistedFilenameOwnership(t *testing.T) {
	t.Parallel()

	provider := newSource(photo("asset-1", testCollisionFilename), photo("asset-2", testPhotoFilename))
	input := request(t.TempDir())
	result, err := newEngine(t, provider).Run(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}

	manifest := readCollisionManifest(t, result.StatePath)
	manifest.Resources[1].RelativePath = testPhotoFilename
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(result.StatePath, data, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	restart, err := newEngine(t, provider).Run(t.Context(), input)

	var failure *photosync.SyncError

	if !errors.As(err, &failure) || restart != nil || provider.visits != 1 || len(provider.downloads) != 2 {
		t.Fatalf("ambiguous ownership accepted: %+v %v downloads=%v", restart, err, provider.downloads)
	}
}
