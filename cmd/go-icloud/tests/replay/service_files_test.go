package replay_test

import (
	"fmt"
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
		return nil, fmt.Errorf("open replay directory: %w", err)
	}

	t.Cleanup(func() {
		closeErr := directory.Close()
		if closeErr != nil {
			t.Errorf("close replay directory: %v", closeErr)
		}
	})

	data, err := directory.ReadFile(filepath.Base(path))
	if err != nil {
		return data, fmt.Errorf("read replay file: %w", err)
	}

	return data, nil
}
