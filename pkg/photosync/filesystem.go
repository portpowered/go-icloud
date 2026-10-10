package photosync

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// OSFileSystem uses native files and durable same-directory atomic replacement.
// The engine confines resource accesses to the resolved output root, including existing symlinks.
type OSFileSystem struct{}

const (
	readLocalFileError    = "read local file: %w"
	inspectLocalFileError = "inspect local file: %w"
	removeLocalFileError  = "remove local file: %w"
	resolveLocalPathError = "resolve local path: %w"
)

// ReadFile reads one local file after checking cancellation.
func (OSFileSystem) ReadFile(ctx context.Context, path string) ([]byte, error) {
	err := ctx.Err()
	if err != nil {
		return nil, fmt.Errorf(readLocalFileError, err)
	}

	data, err := os.ReadFile(path) // #nosec G304 -- caller-owned filesystem; engine confines resource paths.
	if err != nil {
		return nil, fmt.Errorf(readLocalFileError, err)
	}

	return data, nil
}

// Size returns the size of a regular file; directories and symlinks cannot be treated as current content.
func (OSFileSystem) Size(ctx context.Context, path string) (int64, error) {
	err := ctx.Err()
	if err != nil {
		return 0, fmt.Errorf(inspectLocalFileError, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf(inspectLocalFileError, err)
	}

	if !info.Mode().IsRegular() {
		return 0, fmt.Errorf(inspectLocalFileError, fs.ErrInvalid)
	}

	return info.Size(), nil
}

// Remove deletes one file; nonexistent files already satisfy the desired state.
func (OSFileSystem) Remove(ctx context.Context, path string) error {
	err := ctx.Err()
	if err != nil {
		return fmt.Errorf(removeLocalFileError, err)
	}

	err = os.Remove(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf(removeLocalFileError, err)
	}

	return nil
}

// Resolve resolves existing symlink ancestors even when the final file does not exist.
func (OSFileSystem) Resolve(ctx context.Context, path string) (string, error) {
	err := ctx.Err()
	if err != nil {
		return "", fmt.Errorf(resolveLocalPathError, err)
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf(resolveLocalPathError, err)
	}

	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return resolved, nil
	}

	if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf(resolveLocalPathError, err)
	}

	parent := filepath.Dir(absolute)
	if parent == absolute {
		return absolute, nil
	}

	resolved, err = (OSFileSystem{}).Resolve(ctx, parent)
	if err != nil {
		return "", err
	}

	return filepath.Join(resolved, filepath.Base(absolute)), nil
}

// WriteAtomic syncs a temporary file and closes it before replacing its destination.
func (OSFileSystem) WriteAtomic(ctx context.Context, path string, data []byte) error {
	err := ctx.Err()
	if err != nil {
		return fmt.Errorf("write local file: %w", err)
	}

	err = os.MkdirAll(filepath.Dir(path), privateDirectoryMode)
	if err != nil {
		return fmt.Errorf("create local directory: %w", err)
	}

	file, err := os.CreateTemp(filepath.Dir(path), ".go-icloud-sync-*")
	if err != nil {
		return fmt.Errorf("create atomic file: %w", err)
	}

	defer func() { _ = os.Remove(file.Name()) }()

	err = writeAndClose(ctx, file, data)
	if err != nil {
		return err
	}

	err = os.Rename(file.Name(), path)
	if err != nil {
		return fmt.Errorf("replace local file: %w", err)
	}

	return nil
}

const privateDirectoryMode = 0o700

func writeAndClose(ctx context.Context, file *os.File, data []byte) error {
	_, err := file.Write(data)
	if err == nil {
		err = file.Sync()
	}

	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}

	if err == nil {
		err = ctx.Err()
	}

	if err != nil {
		return fmt.Errorf("write atomic file: %w", err)
	}

	return nil
}
