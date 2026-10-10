package photosync_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/internal/photomaterialize"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/pkg/photosync"
)

type exifSizeSource struct {
	*source

	data []byte
}

func (provider *exifSizeSource) Download(_ context.Context, _ icloud.AuthContext, asset photosync.Asset,
	key string,
) ([]byte, bool, error) {
	provider.downloads = append(provider.downloads, asset.ID+":"+key)

	return bytes.Clone(provider.data), true, nil
}

// These minimal JPEG marker streams are synthetic materialization controls.
func TestEXIFMaterializedSizeAndProviderIdentity(t *testing.T) {
	t.Parallel()

	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "new-manifest", true: "legacy-manifest"}[legacy], func(t *testing.T) {
			t.Parallel()

			provider := &exifSizeSource{source: newSource(photo("exif", testPhotoFilename)),
				data: []byte{0xff, 0xd8, 0xff, 0xd9}}
			setEXIFProviderSize(provider)

			input := request(t.TempDir())
			input.Options.SetExifDatetime = !legacy

			first, err := newEngine(t, provider).Run(t.Context(), input)
			if err != nil {
				t.Fatal(err)
			}

			if legacy {
				manifest := readEXIFManifest(t, first.StatePath)
				manifest.Resources[0].LocalSize = nil
				writeEXIFManifest(t, first.StatePath, manifest)
			}

			input.Options.SetExifDatetime = true
			for range 2 {
				result, err := newEngine(t, provider).Run(t.Context(), input)
				if err != nil || result.DownloadedCount != 0 || result.SkippedCount != 1 || result.ShortCircuited {
					t.Fatalf("unchanged EXIF resource downloaded again: %+v, %v", result, err)
				}
			}

			checkEXIFMaterializedSize(t, provider, first.StatePath, input.Options.Directory)

			if len(provider.downloads) != 1 {
				t.Fatal("unchanged provider caused an additional download")
			}

			provider.data = []byte{0xff, 0xd8, 0xff, 0xe0, 0, 2, 0xff, 0xd9}
			setEXIFProviderSize(provider)

			changed, err := newEngine(t, provider).Run(t.Context(), input)
			if err != nil || changed.DownloadedCount != 1 || len(provider.downloads) != 2 {
				t.Fatalf("changed provider size without checksum did not download: %+v, %v", changed, err)
			}

			checkEXIFMaterializedSize(t, provider, first.StatePath, input.Options.Directory)
			input.Options.SetExifDatetime = false

			restarted, err := newEngine(t, provider).Run(t.Context(), input)
			if err != nil || !restarted.ShortCircuited || len(provider.downloads) != 2 {
				t.Fatalf("restart compared transformed bytes with provider size: %+v, %v", restarted, err)
			}
		})
	}
}

func TestLegacyEXIFManifestRequiresRecordedMaterializedSize(t *testing.T) {
	t.Parallel()

	provider := &exifSizeSource{source: newSource(photo("exif", testPhotoFilename)), data: []byte{0xff, 0xd8, 0xff, 0xd9}}
	setEXIFProviderSize(provider)

	input := request(t.TempDir())
	input.Options.SetExifDatetime = true

	first, err := newEngine(t, provider).Run(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}

	manifest := readEXIFManifest(t, first.StatePath)
	manifest.Resources[0].LocalSize = nil
	writeEXIFManifest(t, first.StatePath, manifest)

	second, err := newEngine(t, provider).Run(t.Context(), input)
	if err != nil || second.DownloadedCount != 1 || len(provider.downloads) != 2 {
		t.Fatalf("unrecorded transformed local size was trusted: %+v, %v", second, err)
	}

	checkEXIFMaterializedSize(t, provider, first.StatePath, input.Options.Directory)

	third, err := newEngine(t, provider).Run(t.Context(), input)
	if err != nil || third.DownloadedCount != 0 || len(provider.downloads) != 2 {
		t.Fatalf("upgraded manifest did not reuse recorded size: %+v, %v", third, err)
	}
}

func setEXIFProviderSize(provider *exifSizeSource) {
	resource := provider.assets[0].Resources[testOriginalVersion]
	size := int64(len(provider.data))
	resource.Size, resource.Checksum = &size, nil
	provider.assets[0].Resources[testOriginalVersion] = resource
}

func checkEXIFMaterializedSize(t *testing.T, provider *exifSizeSource, statePath, directory string) {
	t.Helper()

	data, err := readPhotoSyncTestFile(t, filepath.Join(directory, testPhotoFilename))
	if err != nil {
		t.Fatal(err)
	}

	want := photomaterialize.UpdateEXIF(provider.data, *provider.assets[0].TakenAt)
	if !bytes.Equal(data, want) || len(data) <= len(provider.data) ||
		!bytes.Contains(data, []byte("2026:04:01 00:00:00")) {
		t.Fatal("materialized EXIF timestamp or original JPEG changed")
	}

	entry := readEXIFManifest(t, statePath).Resources[0]
	if entry.Size == nil || *entry.Size != int64(len(provider.data)) ||
		entry.LocalSize == nil || *entry.LocalSize != int64(len(data)) {
		t.Fatalf("provider and materialized sizes were conflated: %+v", entry)
	}
}

func readEXIFManifest(t *testing.T, path string) photosync.Manifest {
	t.Helper()

	data, err := readPhotoSyncTestFile(t, path)
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

func writeEXIFManifest(t *testing.T, path string, manifest photosync.Manifest) {
	t.Helper()

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		t.Fatal(err)
	}
}
