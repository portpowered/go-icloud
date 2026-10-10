package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type publicPackageStatus uint8

const (
	publicPackagePresent publicPackageStatus = iota
	publicPackageAdded
	publicPackageRemoved
)

// Include both trees: a deleted public package must not disappear from the
// compatibility denominator merely because the release tree cannot list it.
func expandPublicPackages(root, baseline string, configured []string) ([]string, error) {
	if len(configured) != 1 || configured[0] != defaultPublicPackages {
		return configured, nil
	}
	packages := map[string]bool{}
	for _, tree := range []string{root, baseline} {
		err := filepath.WalkDir(filepath.Join(tree, "pkg"), func(path string, entry fs.DirEntry, walkErr error) error {
			if errors.Is(walkErr, fs.ErrNotExist) {
				return nil
			}
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			relative, err := filepath.Rel(tree, filepath.Dir(path))
			if err != nil {
				return newGateError("resolve public package path", err)
			}
			packages[filepath.ToSlash(relative)] = true
			return nil
		})
		if err != nil {
			return nil, newGateError("inventory public packages", err)
		}
	}
	if len(packages) == 0 {
		return nil, newGateError("public package inventory is empty", nil)
	}
	result := make([]string, 0, len(packages))
	for path := range packages {
		result = append(result, path)
	}
	slices.Sort(result)
	return result, nil
}

func publicPackagePresence(current, baseline, path string) (publicPackageStatus, error) {
	newExists, err := publicPackageExists(filepath.Join(current, path))
	if err != nil {
		return publicPackagePresent, err
	}
	oldExists, err := publicPackageExists(filepath.Join(baseline, path))
	if err != nil {
		return publicPackagePresent, err
	}

	switch {
	case !oldExists && newExists:
		return publicPackageAdded, nil
	case oldExists && !newExists:
		return publicPackageRemoved, nil
	case !oldExists && !newExists:
		return publicPackagePresent, newGateError("configured public package does not exist: "+path, nil)
	default:
		return publicPackagePresent, nil
	}
}

func publicPackageExists(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, newGateError("read public package", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			return true, nil
		}
	}
	return false, nil
}
