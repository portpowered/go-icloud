package photosync

import (
	"encoding/json"
	"fmt"

	"github.com/portpowered/go-icloud/internal/photomaterialize"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

func projectAsset(photo icloud.Photo, album string, library *icloud.PhotoLibrary) (Asset, error) {
	asset := new(Asset)
	asset.ID, asset.Filename = photo.ID, photo.Filename
	asset.Album, asset.Library = &album, library
	asset.ItemType = AssetItemType(photo.ItemType)
	asset.IsLivePhoto = photo.IsLivePhoto
	asset.AddedAt, asset.TakenAt = &photo.Added, &photo.Created
	asset.Metadata = photomaterialize.ExtractMetadataJSON(photo.AssetMetadata)

	asset.Resources = make(map[string]Resource, len(photo.Versions))

	for key, version := range photo.Versions {
		resource, err := projectResource(key, version)
		if err != nil {
			return *asset, err
		}

		asset.Resources[key] = resource
	}

	return *asset, nil
}

func projectResource(key string, version icloud.PhotoResource) (Resource, error) {
	resource := new(Resource)
	resource.Key, resource.Filename = key, version.Filename

	fields := []projectionField{
		{data: version.Url, target: &resource.Url},
		{data: version.Size, target: &resource.Size},
		{data: version.Type, target: &resource.Type},
		{data: version.Checksum, target: &resource.Checksum},
	}
	for _, field := range fields {
		if len(field.data) == 0 {
			continue
		}

		err := json.Unmarshal(field.data, field.target)
		if err != nil {
			return *resource, fmt.Errorf("decode photo resource projection: %w", err)
		}
	}

	return *resource, nil
}

type projectionField struct {
	data   json.RawMessage
	target any
}
