package photosync_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func readPhotoSyncTestFile(t *testing.T, path string) ([]byte, error) {
	t.Helper()

	directory, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("open photo sync test directory: %w", err)
	}

	t.Cleanup(func() {
		err := directory.Close()
		if err != nil {
			t.Error(err)
		}
	})

	data, err := directory.ReadFile(filepath.Base(path))
	if err != nil {
		return nil, fmt.Errorf("read photo sync test file: %w", err)
	}

	return data, nil
}
