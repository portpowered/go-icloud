package photosync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
)

var errManifest = errors.New("invalid or mismatched photo sync manifest")

func (engine *Engine) loadManifest(ctx context.Context, path, key string) (Manifest, error) {
	manifest := Manifest{TargetKey: key, Cursor: nil, Resources: []SyncedResource{}}

	data, err := engine.files.ReadFile(ctx, path)

	if errors.Is(err, fs.ErrNotExist) {
		return manifest, nil
	}

	if err != nil {
		return manifest, fmt.Errorf("load photo sync manifest: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	err = decoder.Decode(&manifest)
	if err != nil {
		return manifest, fmt.Errorf("decode photo sync manifest: %w", err)
	}

	err = decoder.Decode(new(any))

	if !errors.Is(err, io.EOF) || manifest.TargetKey != key || manifest.Resources == nil {
		return manifest, errManifest
	}

	return manifest, validateManifestResources(manifest.Resources)
}

func validateManifestResources(resources []SyncedResource) error {
	seen := map[resourceID]bool{}

	for _, resource := range resources {
		identity := resourceID{asset: resource.AssetID, key: resource.ResourceKey}
		if seen[identity] || resource.AssetID == "" || resource.ResourceKey == "" || resource.RelativePath == "" ||
			resource.Size != nil && *resource.Size < 0 || resource.LocalSize != nil && *resource.LocalSize < 0 {
			return errManifest
		}

		seen[identity] = true
	}

	return nil
}

func (engine *Engine) saveManifest(ctx context.Context, path string, manifest Manifest) error {
	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode photo sync manifest: %w", err)
	}

	err = engine.files.WriteAtomic(ctx, path, data)
	if err != nil {
		return fmt.Errorf("save photo sync manifest: %w", err)
	}

	return nil
}

type resourceID struct{ asset, key string }

func manifestResource(manifest Manifest, id resourceID) *SyncedResource {
	for index := range manifest.Resources {
		resource := &manifest.Resources[index]
		if resource.AssetID == id.asset && resource.ResourceKey == id.key {
			return resource
		}
	}

	return nil
}

func upsertResource(manifest *Manifest, resource SyncedResource) {
	previous := manifestResource(*manifest, resourceID{asset: resource.AssetID, key: resource.ResourceKey})
	if previous != nil {
		*previous = resource

		return
	}

	manifest.Resources = append(manifest.Resources, resource)
}
