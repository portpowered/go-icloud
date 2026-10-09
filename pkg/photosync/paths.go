package photosync

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	errUnsafePath   = errors.New("photo path escapes destination")
	errFolderFormat = errors.New("unsupported folder date format")
	unsafeComponent = regexp.MustCompile(`[\x00-\x1f/\\:]+`)
)

func safeComponent(value, fallback string) string {
	value = strings.TrimSpace(unsafeComponent.ReplaceAllString(value, "-"))
	if value == "" || value == "." || value == ".." {
		return fallback
	}

	return value
}

func relativePath(asset Asset, resource Resource, format string) (string, error) {
	filename := safeComponent(path.Base(strings.ReplaceAll(resource.Filename, `\`, "/")), "photo")
	if format == "none" || format == "" {
		return filename, nil
	}

	instant := time.Unix(0, 0).UTC()
	if asset.TakenAt != nil {
		instant = *asset.TakenAt
	}

	folder, err := formatFolder(instant, format)
	if err != nil {
		return "", err
	}

	parts := []string{}

	for part := range strings.SplitSeq(strings.ReplaceAll(folder, `\`, "/"), "/") {
		if part != "" && part != "." && part != ".." {
			parts = append(parts, safeComponent(part, "folder"))
		}
	}

	parts = append(parts, filename)

	return path.Join(parts...), nil
}

func uniquePath(candidate, assetID, key string, reserved map[string]bool, tracked map[string]resourceID) string {
	owner, occupied := tracked[candidate]
	if !reserved[candidate] && (!occupied || owner == (resourceID{asset: assetID, key: key})) {
		return candidate
	}

	identifier := []rune(assetID)
	if len(identifier) > identifierPrefixLength {
		identifier = identifier[:identifierPrefixLength]
	}

	discriminator := string(identifier)

	extension := path.Ext(candidate)
	stem := strings.TrimSuffix(candidate, extension)

	for index := 1; ; index++ {
		suffix := "_" + discriminator
		if index > 1 {
			suffix += fmt.Sprintf("_%d", index)
		}

		result := stem + suffix + extension

		owner, occupied = tracked[result]

		if !reserved[result] && (!occupied || owner == (resourceID{asset: assetID, key: key})) {
			return result
		}
	}
}

const identifierPrefixLength = 8

func (engine *Engine) targetPath(ctx context.Context, root, relative string) (string, error) {
	if relative == "" || relative == "." || filepath.IsAbs(relative) || strings.Contains(relative, `\`) {
		return "", errUnsafePath
	}

	target, err := engine.files.Resolve(ctx, filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return "", fmt.Errorf("resolve photo target: %w", err)
	}

	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", errUnsafePath
	}

	return target, nil
}
