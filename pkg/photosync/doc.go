// Package photosync materializes iCloud Photos through injectable storage and source interfaces.
//
// Each run uses request-scoped credentials, persists a synchronization manifest,
// and returns download, deletion, and preview results. Source-compatible date
// formatting, RAW alignment, XMP sidecars, and missing EXIF timestamps preserve
// local filenames and metadata without embedding transport mechanics.
package photosync
