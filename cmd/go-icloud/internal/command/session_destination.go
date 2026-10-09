package command

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var errReferenceDestination = errors.New("native session destination is an imported reference file")

func preserveReferenceFiles(destination string, sources []string) error {
	if len(sources) == 0 {
		return nil
	}

	output, err := os.Stat(filepath.Clean(destination))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("inspect session destination: %w", err)
	}

	for _, source := range sources {
		input, statErr := os.Stat(filepath.Clean(source))
		if statErr != nil {
			return fmt.Errorf("inspect reference source: %w", statErr)
		}

		if os.SameFile(input, output) {
			return errReferenceDestination
		}
	}

	return nil
}
