package photosync

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/portpowered/go-icloud/internal/photomaterialize"
)

// Metadata contains the decoded metadata used to create an XMP sidecar.
type Metadata = photomaterialize.Metadata

const measureMaterializedPhotoError = "measure materialized photo: %w"

func (state *runState) materialize(ctx context.Context, asset Asset, resource Resource) (bool, bool, string, error) {
	relative, err := relativePath(asset, resource, state.request.Options.FolderStructure)
	if err != nil {
		return false, false, "", err
	}

	relative = uniquePath(relative, asset.ID, resource.Key, state.reserved, state.tracked)
	state.reserved[pathIdentity(relative)] = true
	identity := resourceID{asset: asset.ID, key: resource.Key}
	state.current[identity] = true

	target, err := state.engine.targetPath(ctx, state.root, relative)
	if err != nil {
		if !errors.Is(err, errUnsafePath) {
			return false, false, relative, err
		}

		state.complete, state.consecutive = false, 0
		state.addItem(asset.ID, resource.Key, relative, Skipped, reason(UnsafePath))

		return skippedUnsafe(relative)
	}

	previous := manifestResource(state.manifest, identity)
	if state.engine.isCurrent(ctx, target, previous, resource, relative) {
		state.addItem(asset.ID, resource.Key, relative, Skipped, reason(AlreadyCurrent))

		state.consecutive++

		err = state.localMetadata(ctx, target, asset, resource)
		if err == nil && !state.preview() {
			err = state.refreshLocalSize(ctx, target, identity)
		}

		return true, true, relative, err
	}

	state.consecutive = 0
	if state.preview() {
		why := PrintOnly
		if state.request.Options.DryRun {
			why = DryRun
		}

		state.addItem(asset.ID, resource.Key, relative, Listed, reason(why))

		return false, false, relative, nil
	}

	return state.download(ctx, asset, resource, target, relative)
}

func skippedUnsafe(relative string) (bool, bool, string, error) { return false, false, relative, nil }

func (state *runState) download(ctx context.Context, asset Asset, resource Resource,
	target, relative string,
) (bool, bool, string, error) {
	data, available, err := state.engine.source.Download(ctx, state.request.Auth, asset, resource.Key)
	if err != nil {
		return false, false, relative, fmt.Errorf("download photo resource: %w", err)
	}

	if !available {
		state.complete = false
		state.addItem(asset.ID, resource.Key, relative, Skipped, reason(MissingDownloadData))

		return false, false, relative, nil
	}

	err = state.engine.files.WriteAtomic(ctx, target, data)
	if err != nil {
		return false, false, relative, fmt.Errorf("materialize photo resource: %w", err)
	}

	err = state.localMetadata(ctx, target, asset, resource)
	if err != nil {
		return false, false, relative, err
	}

	now := state.engine.now().UTC()
	localSize, err := state.engine.files.Size(ctx, target)
	if err != nil {
		return false, false, relative, fmt.Errorf(measureMaterializedPhotoError, err)
	}

	upsertResource(&state.manifest, SyncedResource{AssetID: asset.ID, ResourceKey: resource.Key,
		RelativePath: relative, Size: resource.Size, LocalSize: &localSize, Checksum: resource.Checksum, DownloadedAt: &now})

	err = state.engine.saveManifest(ctx, state.result.StatePath, state.manifest)
	if err != nil {
		return false, false, relative, err
	}

	state.addItem(asset.ID, resource.Key, relative, Downloaded, nil)

	return true, true, relative, nil
}

func (state *runState) refreshLocalSize(ctx context.Context, target string, identity resourceID) error {
	entry := manifestResource(state.manifest, identity)
	localSize, err := state.engine.files.Size(ctx, target)
	if err != nil {
		return fmt.Errorf(measureMaterializedPhotoError, err)
	}

	if entry.LocalSize != nil && *entry.LocalSize == localSize {
		return nil
	}

	entry.LocalSize = &localSize

	return state.engine.saveManifest(ctx, state.result.StatePath, state.manifest)
}

func (state *runState) localMetadata(ctx context.Context, target string, asset Asset, resource Resource) error {
	if state.preview() {
		return nil
	}

	if state.request.Options.SetExifDatetime && asset.TakenAt != nil {
		err := state.writeEXIF(ctx, target, *asset.TakenAt)
		if err != nil {
			return err
		}
	}

	if state.request.Options.XmpSidecar && asset.Metadata != nil && !strings.HasSuffix(resource.Key, "_video") {
		return state.writeSidecar(ctx, target+".xmp", *asset.Metadata)
	}

	return nil
}

func (state *runState) writeEXIF(ctx context.Context, target string, taken time.Time) error {
	extension := strings.ToLower(filepath.Ext(target))
	if extension != ".jpg" && extension != ".jpeg" {
		return nil
	}

	data, err := state.engine.files.ReadFile(ctx, target)
	if err != nil {
		return fmt.Errorf("read photo for EXIF: %w", err)
	}

	updated := photomaterialize.UpdateEXIF(data, taken)

	err = state.engine.files.WriteAtomic(ctx, target, updated)
	if err != nil {
		return fmt.Errorf("write photo EXIF: %w", err)
	}

	return nil
}

func reason(value ItemReason) *ItemReason { return &value }

func (state *runState) addItem(assetID, key, path string, action ItemAction, why *ItemReason) {
	state.result.Items = append(state.result.Items,
		Item{AssetID: assetID, ResourceKey: key, Path: path, Action: action, Reason: why})

	switch action {
	case Downloaded:
		state.result.DownloadedCount++
	case Skipped:
		state.result.SkippedCount++
	case Deleted:
		state.result.DeletedCount++
	case Listed:
		state.result.ListedCount++
	}
}
