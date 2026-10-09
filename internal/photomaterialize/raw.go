package photomaterialize

import (
	"path/filepath"
	"strings"
)

// ResourceIsRAW recognizes Source RAW content types and filename extensions.
func ResourceIsRAW(filename, kind string) bool {
	if strings.Contains(strings.ToLower(kind), "raw") {
		return true
	}

	switch strings.ToLower(filepath.Ext(filename)) {
	case ".arw", ".cr2", ".cr3", ".crw", ".dng", ".nef", ".nrf", ".nrw", ".orf", ".pef", ".raf", ".rw2":
		return true
	default:
		return false
	}
}

// ShouldSwapRAW selects whether two existing resources exchange roles.
func ShouldSwapRAW(originalName, originalType, alternativeName, alternativeType, policy string) bool {
	original := ResourceIsRAW(originalName, originalType)
	alternative := ResourceIsRAW(alternativeName, alternativeType)

	return policy == "original" && alternative && !original || policy == "alternative" && original && !alternative
}
