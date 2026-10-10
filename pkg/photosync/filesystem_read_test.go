package photosync_test

import (
	"os"
	"path/filepath"
	"testing"
)

func readPhotoSyncTestFile(t *testing.T, path string) ([]byte, error) {
	t.Helper()

	directory, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}

	t.Cleanup(func() {
		err := directory.Close()
		if err != nil {
			t.Error(err)
		}
	})

	return directory.ReadFile(filepath.Base(path))
}
