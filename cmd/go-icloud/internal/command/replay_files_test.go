package command_test

import (
	"os"
	"path/filepath"
	"testing"
)

// readReplayFile confines private result and synthetic fixture reads to their
// owning directory while preserving the bytes and filesystem error (LIB-05).
func readReplayFile(t *testing.T, path string) ([]byte, error) {
	t.Helper()

	directory, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}

	t.Cleanup(func() {
		closeErr := directory.Close()
		if closeErr != nil {
			t.Errorf("close replay directory: %v", closeErr)
		}
	})

	return directory.ReadFile(filepath.Base(path))
}
