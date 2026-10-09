package photosync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
)

// DefaultOptions returns Source defaults without selecting any account.
func DefaultOptions(directory string) Options {
	return Options{Directory: directory, StateDir: nil, Library: string(RootLibrary), Albums: []string{},
		Size: OptionsSizeOriginal, LivePhotoSize: OptionsLivePhotoSizeOriginal,
		FolderStructure: "none", Recent: nil, UntilFound: nil, SkipVideos: false,
		SkipLivePhotos: false, AlignRaw: OptionsAlignRawAsIs, XmpSidecar: false,
		SetExifDatetime: false, KeepIcloudRecentDays: nil, OnlyPrintFilenames: false,
		DryRun: false, AutoDelete: false}
}

func validateOptions(options Options) error {
	if options.Directory == "" || options.Library == string(LegacySharedLibrary) || !options.Size.Valid() ||
		!options.LivePhotoSize.Valid() || !options.AlignRaw.Valid() {
		return errOptions
	}

	return validateLimits(options)
}

func validateLimits(options Options) error {
	if options.UntilFound != nil && (*options.UntilFound < 1 || options.AutoDelete ||
		options.KeepIcloudRecentDays != nil) {
		return errOptions
	}

	if options.Recent != nil && *options.Recent < 1 {
		return errOptions
	}

	if options.KeepIcloudRecentDays != nil && *options.KeepIcloudRecentDays < 0 {
		return errOptions
	}

	return nil
}

func normalizedAlbums(albums []string) []string {
	result := make([]string, 0, len(albums))

	for _, album := range albums {
		if album != "" {
			result = append(result, album)
		}
	}

	slices.Sort(result)

	return result
}

func targetIdentity(request Request, directory string) (string, error) {
	options := request.Options
	identity := TargetIdentity{AccountID: request.Auth.AccountID, Library: options.Library,
		Albums: normalizedAlbums(options.Albums), Directory: directory, Size: string(options.Size),
		LivePhotoSize: string(options.LivePhotoSize), FolderStructure: options.FolderStructure,
		Recent: options.Recent, SkipVideos: options.SkipVideos, SkipLivePhotos: options.SkipLivePhotos,
		AlignRaw: string(options.AlignRaw)}

	data, err := json.Marshal(identity)
	if err != nil {
		return "", fmt.Errorf("encode photo sync identity: %w", err)
	}

	digest := sha256.Sum256(data)

	return hex.EncodeToString(digest[:]), nil
}

func manifestPath(options Options, key string) string {
	root := filepath.Join(options.Directory, ".go-icloud-state")
	if options.StateDir != nil {
		root = *options.StateDir
	}

	return filepath.Join(root, key+".json")
}

func copyRequest(request Request) (Request, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return request, fmt.Errorf("copy photo sync request: %w", err)
	}

	var copied Request

	err = json.Unmarshal(data, &copied)
	if err != nil {
		return request, fmt.Errorf("copy photo sync request: %w", err)
	}

	return copied, nil
}
