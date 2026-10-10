package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func checkContent(pages map[string]page, file string) error {
	data, err := os.ReadFile(filepath.Clean(file))
	if err != nil {
		return fmt.Errorf("read content expectations: %w", err)
	}

	var expectations map[string][]string

	err = json.Unmarshal(data, &expectations)
	if err != nil {
		return fmt.Errorf("decode content expectations: %w", err)
	}

	if len(expectations) == 0 {
		return fmt.Errorf("%w: %s", errEmptyExpectations, file)
	}

	for address, terms := range expectations {
		rendered, exists := pages[address]
		if !exists {
			return fmt.Errorf("%w: %s", errExpectedPage, address)
		}

		for _, term := range terms {
			if term == "" || !strings.Contains(rendered.text, term) {
				return fmt.Errorf("%w on %s: %q", errExpectedContent, address, term)
			}
		}
	}

	return nil
}
