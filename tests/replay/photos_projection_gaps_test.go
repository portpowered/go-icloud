package replay_test

import (
	"path/filepath"
	"testing"
)

func TestPhotosFallbackProjectionPairedReplay(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"raw-original-format", "raw-filename-extension", "image-filename-extension", "unknown-movie-format",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runPhotoAssetsSDK(t, filepath.Join(replayExpectedFixturesSyntheticHTTP, "photos-replay-gap-"+name+".json"))
		})
	}
}

func TestPhotosCursorPaginationPairedReplay(t *testing.T) {
	t.Parallel()

	path := filepath.Join(replayExpectedFixturesSyntheticHTTP, "photos-replay-gap-cursor-pagination.json")
	runPhotoChangesSDK(t, path, readAccountScenario(t, path))
}
