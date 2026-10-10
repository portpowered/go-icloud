package photosync

import (
	"context"
	"github.com/portpowered/go-icloud/internal/photomaterialize"
	"maps"
)

const (
	originalResourceKey    = "original"
	alternativeResourceKey = "alternative"
)

func selectResources(asset Asset, options Options) []Resource {
	if skipAsset(asset, options) {
		return nil
	}

	resources := copyResources(asset.Resources)
	alignResources(resources, options.AlignRaw)

	result := []Resource{}

	candidates := []string{string(options.Size), originalResourceKey, "medium", "thumb"}

	if resource := resolveResource(resources, candidates); resource != nil {
		result = append(result, *resource)
	}

	if asset.IsLivePhoto && !options.SkipVideos {
		candidates := []string{string(options.LivePhotoSize) + "_video", "original_video", "medium_video", "thumb_video"}
		if resource := resolveResource(resources, candidates); resource != nil &&
			(len(result) == 0 || result[0].Key != resource.Key) {
			result = append(result, *resource)
		}
	}

	return result
}

func skipAsset(asset Asset, options Options) bool {
	return asset.ItemType == Movie && options.SkipVideos || asset.IsLivePhoto && options.SkipLivePhotos
}

func copyResources(resources map[string]Resource) map[string]Resource {
	result := make(map[string]Resource, len(resources))
	maps.Copy(result, resources)

	return result
}

func resolveResource(resources map[string]Resource, candidates []string) *Resource {
	for _, key := range candidates {
		resource, ok := resources[key]
		if ok && resource.Url != nil && *resource.Url != "" {
			if resource.Key == "" {
				resource.Key = key
			}

			return &resource
		}
	}

	return nil
}

func alignResources(resources map[string]Resource, policy OptionsAlignRaw) {
	original, hasOriginal := resources[originalResourceKey]

	alternative, hasAlternative := resources[alternativeResourceKey]

	if !hasOriginal || !hasAlternative {
		return
	}

	if photomaterialize.ShouldSwapRAW(original.Filename, stringValue(original.Type),
		alternative.Filename, stringValue(alternative.Type), string(policy)) {
		resources[originalResourceKey], resources[alternativeResourceKey] = alternative, original
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func (engine *Engine) isCurrent(ctx context.Context, target string, previous *SyncedResource,
	resource Resource, relative string,
) bool {
	if previous == nil || previous.RelativePath != relative {
		return false
	}

	if resource.Size != nil && (previous.Size == nil || *resource.Size != *previous.Size) {
		return false
	}

	size, err := engine.files.Size(ctx, target)
	if err != nil || !materializedSizeMatches(*previous, size) {
		return false
	}

	return resource.Checksum == nil || previous.Checksum == nil || *resource.Checksum == *previous.Checksum
}

func materializedSizeMatches(resource SyncedResource, size int64) bool {
	expected := resource.LocalSize
	if expected == nil {
		expected = resource.Size
	}

	return expected == nil || *expected == size
}
