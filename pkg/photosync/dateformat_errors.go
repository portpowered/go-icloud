package photosync

import "fmt"

// UnsupportedDateFormatError identifies a valid Python format whose output depends on its runtime.
type UnsupportedDateFormatError struct {
	Feature string
}

// Error explains which runtime-specific behavior cannot be reproduced.
func (failure *UnsupportedDateFormatError) Error() string {
	return fmt.Sprintf("unsupported Python date formatting feature %q", failure.Feature)
}

// Unwrap preserves the folder-format failure classification.
func (failure *UnsupportedDateFormatError) Unwrap() error {
	return errFolderFormat
}

func unsupportedDateFormat(feature string) error {
	return &UnsupportedDateFormatError{Feature: feature}
}
