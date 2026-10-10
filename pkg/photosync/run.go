package photosync

import (
	"context"
	"fmt"
	"time"
)

type runState struct {
	engine      *Engine
	request     Request
	root        string
	manifest    Manifest
	result      Result
	tracked     map[string]resourceID
	reserved    map[string]bool
	current     map[resourceID]bool
	seenAssets  map[string]bool
	consecutive int
	complete    bool
	now         time.Time
}

// Run reconciles one selected account target, persisting only successfully written resources.
// Dry-run and print-only runs never write files, delete remote assets or advance persistent state.
func (engine *Engine) Run(ctx context.Context, request Request) (*Result, error) {
	err := ctx.Err()
	if err != nil {
		return nil, &SyncError{Operation: runOperation, Cause: err}
	}

	request, err = copyRequest(request)
	if err != nil {
		return nil, &SyncError{Operation: runOperation, Cause: err}
	}

	err = validateOptions(request.Options)
	if err != nil {
		return nil, &SyncError{Operation: runOperation, Cause: err}
	}

	if !engine.running.CompareAndSwap(false, true) {
		return nil, &SyncError{Operation: runOperation, Cause: errBusy}
	}

	defer engine.running.Store(false)

	state, err := engine.prepareRun(ctx, request)
	if err != nil {
		return nil, &SyncError{Operation: runOperation, Cause: err}
	}

	if state.shortCircuit(ctx) {
		state.result.ShortCircuited = true

		return &state.result, nil
	}

	if !state.validRecentCutoff() {
		return &state.result, &SyncError{Operation: runOperation, Cause: errOptions}
	}

	err = engine.source.Visit(ctx, request.Auth, request.Options, func(asset Asset) (bool, error) {
		return state.visit(ctx, asset)
	})
	if err == nil {
		err = state.finish(ctx)
	}

	if err != nil {
		return &state.result, &SyncError{Operation: runOperation, Cause: err}
	}

	return &state.result, nil
}

func (state *runState) finish(ctx context.Context) error {
	if state.preview() {
		return nil
	}

	if state.request.Options.AutoDelete {
		err := state.removeStale(ctx)
		if err != nil {
			return err
		}
	}

	if !state.complete {
		return nil
	}

	state.manifest.Cursor = state.result.SyncCursor

	return state.engine.saveManifest(ctx, state.result.StatePath, state.manifest)
}

func (engine *Engine) prepareRun(ctx context.Context, request Request) (*runState, error) {
	root, err := engine.files.Resolve(ctx, request.Options.Directory)
	if err != nil {
		return nil, fmt.Errorf("resolve sync directory: %w", err)
	}

	key, err := targetIdentity(request, root)
	if err != nil {
		return nil, err
	}

	statePath := manifestPath(request.Options, key)

	manifest, err := engine.loadManifest(ctx, statePath, key)
	if err != nil {
		return nil, err
	}

	cursor, err := engine.source.Cursor(ctx, request.Auth, request.Options)
	if err != nil {
		return nil, fmt.Errorf("read photo cursor: %w", err)
	}

	result := Result{Directory: request.Options.Directory, StatePath: statePath, Library: request.Options.Library,
		Albums: normalizedAlbums(request.Options.Albums), SyncCursor: cursor, ShortCircuited: false,
		DownloadedCount: 0, SkippedCount: 0, DeletedCount: 0, ListedCount: 0, Items: []Item{}}

	state := &runState{engine: engine, request: request, root: root, manifest: manifest, result: result,
		tracked: map[string]resourceID{}, reserved: map[string]bool{}, current: map[resourceID]bool{},
		seenAssets: map[string]bool{}, consecutive: 0, complete: true, now: engine.now()}
	for _, resource := range manifest.Resources {
		state.tracked[resource.RelativePath] = resourceID{asset: resource.AssetID, key: resource.ResourceKey}
	}

	return state, nil
}

func (state *runState) preview() bool {
	return state.request.Options.DryRun || state.request.Options.OnlyPrintFilenames
}

func (state *runState) shortCircuit(ctx context.Context) bool {
	options := state.request.Options
	if disablesShortCircuit(options) || !sameCursor(state.result.SyncCursor, state.manifest.Cursor) ||
		len(state.manifest.Resources) == 0 {
		return false
	}

	for _, entry := range state.manifest.Resources {
		path, err := state.engine.targetPath(ctx, state.root, entry.RelativePath)
		if err != nil {
			return false
		}

		size, err := state.engine.files.Size(ctx, path)
		if err != nil || !materializedSizeMatches(entry, size) {
			return false
		}
	}

	return true
}

func disablesShortCircuit(options Options) bool {
	return options.AutoDelete || options.DryRun || options.OnlyPrintFilenames ||
		options.XmpSidecar || options.SetExifDatetime || options.KeepIcloudRecentDays != nil
}

func sameCursor(first, second *string) bool {
	return first != nil && second != nil && *first == *second
}

func (state *runState) visit(ctx context.Context, asset Asset) (bool, error) {
	err := ctx.Err()
	if err != nil {
		return false, fmt.Errorf("visit photo asset: %w", err)
	}

	if state.seenAssets[asset.ID] {
		return true, nil
	}

	state.seenAssets[asset.ID] = true
	if !state.includeAsset(asset) {
		return true, nil
	}

	resources := selectResources(asset, state.request.Options)
	ready, confirmed := len(resources) > 0, false
	paths := []string{}

	for _, resource := range resources {
		local, complete, path, err := state.materialize(ctx, asset, resource)
		if err != nil {
			return false, err
		}

		ready = ready && complete
		confirmed = confirmed || local

		paths = append(paths, path)

		if state.untilFound() {
			break
		}
	}

	err = state.retention(ctx, asset, ready, confirmed, paths)
	if err != nil {
		return false, err
	}

	if state.untilFound() {
		// The remaining assets are unvisited, so this cursor cannot certify the
		// full target even though every visited resource is already current.
		state.complete = false

		return false, nil
	}

	return true, nil
}

func (state *runState) includeAsset(asset Asset) bool {
	if state.request.Options.Recent == nil {
		return true
	}

	added := asset.AddedAt
	if added == nil {
		added = asset.TakenAt
	}

	cutoff := state.now.UTC().AddDate(0, 0, -*state.request.Options.Recent)

	return added != nil && !added.Before(cutoff)
}

func (state *runState) validRecentCutoff() bool {
	const maximumSourceDays = 999999999

	recent := state.request.Options.Recent
	if recent == nil {
		return true
	}

	return *recent <= maximumSourceDays && state.now.UTC().AddDate(0, 0, -*recent).Year() >= 1
}

func (state *runState) untilFound() bool {
	limit := state.request.Options.UntilFound

	return limit != nil && state.consecutive >= *limit
}
