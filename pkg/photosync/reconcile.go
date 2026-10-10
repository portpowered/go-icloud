package photosync

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/portpowered/go-icloud/internal/photomaterialize"
)

const resolvePhotoSidecarError = "resolve photo sidecar: %w"

func (state *runState) writeSidecar(ctx context.Context, path string, metadata Metadata) error {
	relative, err := filepath.Rel(state.root, path)
	if err != nil {
		return fmt.Errorf(resolvePhotoSidecarError, err)
	}

	path, err = state.engine.targetPath(ctx, state.root, filepath.ToSlash(relative))
	if err != nil {
		return fmt.Errorf(resolvePhotoSidecarError, err)
	}

	existing, err := state.engine.files.ReadFile(ctx, path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read photo sidecar: %w", err)
	}

	if err == nil && !photomaterialize.OwnedXMP(existing) {
		return nil
	}

	data := photomaterialize.RenderXMP(metadata)

	err = state.engine.files.WriteAtomic(ctx, path, data)
	if err != nil {
		return fmt.Errorf("write photo sidecar: %w", err)
	}

	return nil
}

func (state *runState) retention(ctx context.Context, asset Asset, ready, confirmed bool, paths []string) error {
	days := state.request.Options.KeepIcloudRecentDays
	if days == nil || state.preview() || !ready || !confirmed || asset.TakenAt == nil {
		return nil
	}

	if !retentionAgeReached(state.now, *asset.TakenAt, *days) {
		return nil
	}

	deleted, err := state.engine.source.Delete(ctx, state.request.Auth, asset)
	state.complete = false

	if err != nil {
		return fmt.Errorf("delete retained remote photo: %w", err)
	}

	if deleted {
		path := asset.Filename
		if len(paths) > 0 {
			path = paths[0]
		}

		state.addItem(asset.ID, "remote", path, Deleted, reason(KeepIcloudRecentDays))
	}

	return nil
}

func retentionAgeReached(now, taken time.Time, days int) bool {
	const secondsPerDay = 86400

	elapsed := now.Unix() - taken.Unix()
	if now.Nanosecond() < taken.Nanosecond() {
		elapsed--
	}

	return elapsed >= 0 && elapsed/secondsPerDay >= int64(days)
}

func (state *runState) removeStale(ctx context.Context) error {
	retained := make([]SyncedResource, 0, len(state.manifest.Resources))

	for _, resource := range state.manifest.Resources {
		if state.current[resourceID{asset: resource.AssetID, key: resource.ResourceKey}] {
			retained = append(retained, resource)

			continue
		}

		target, err := state.engine.targetPath(ctx, state.root, resource.RelativePath)
		if err != nil {
			if !errors.Is(err, errUnsafePath) {
				return err
			}

			state.addItem(resource.AssetID, resource.ResourceKey, resource.RelativePath, Skipped, reason(UnsafePath))

			continue
		}

		err = state.engine.files.Remove(ctx, target)
		if err != nil {
			return fmt.Errorf("remove stale local photo: %w", err)
		}

		state.addItem(resource.AssetID, resource.ResourceKey, resource.RelativePath, Deleted, nil)
	}

	state.manifest.Resources = retained

	return state.engine.saveManifest(ctx, state.result.StatePath, state.manifest)
}
