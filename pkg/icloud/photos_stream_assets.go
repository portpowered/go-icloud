package icloud

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/sharedphotos"
)

const sharedRecordPairSize = 2

func (sdk *SDK) readSharedPhotos(ctx context.Context, auth AuthContext, selector string, photoID *string,
	operation string, complete bool,
) (*photosRead, []SharedPhoto, error) {
	read, albums, err := sdk.beginSharedPhotos(ctx, auth, operation)
	if err != nil {
		return nil, nil, err
	}

	album, found := selectSharedAlbum(albums, selector)
	if !found {
		return nil, nil, read.failure(errPhotoAlbumMissing, NotFound)
	}

	photos, err := read.sharedPages(ctx, album, photoID, complete)
	if err != nil {
		return nil, nil, err
	}

	return read, photos, nil
}

func (read *photosRead) sharedPages(ctx context.Context, album sharedphotos.SharedAlbum, photoID *string,
	complete bool,
) ([]SharedPhoto, error) {
	photos := []SharedPhoto{}
	seen := map[string]bool{}

	for offset := int64(0); ; {
		response, err := read.sdk.web.PhotosSharedAssets(ctx, read.auth, album, offset,
			photoSourcePageSize*sharedRecordPairSize)
		if err != nil {
			return nil, read.failure(err, InvalidResponse)
		}

		read.responses = append(read.responses, response.Metadata)

		records, err := photoNormalRecords(response.Data)
		if err != nil {
			return nil, read.failure(err, InvalidResponse)
		}

		pairs, err := projectSharedPairs(records)
		if err != nil {
			return nil, read.failure(err, InvalidResponse)
		}

		selected, err := projectSharedPage(pairs, seen, photoID, complete)
		if err != nil {
			return nil, read.failure(err, InvalidResponse)
		}

		photos = append(photos, selected...)
		if photoID != nil && len(selected) != 0 {
			return photos, nil
		}

		threshold := photoSourcePageSize / sharedRecordPairSize
		if photoID != nil {
			threshold = photoSourcePageSize
		}

		if len(pairs) < threshold {
			return photos, nil
		}

		offset += int64(len(pairs))
	}
}

func projectSharedPairs(records []cloudkit.CKRecord) ([]photoPair, error) {
	assets := map[string]cloudkit.CKRecord{}
	masters := []cloudkit.CKRecord{}

	for _, record := range records {
		switch cloudkit.PhotoRecordKind(record.RecordType) {
		case cloudkit.CPLAsset:
			value, err := photoFieldValue(record, string(cloudkit.MasterRef))
			if err != nil {
				return nil, err
			}

			var reference cloudkit.CKReference
			if json.Unmarshal(value, &reference) == nil && reference.RecordName != "" {
				assets[reference.RecordName] = record
			}
		case cloudkit.CPLMaster:
			masters = append(masters, record)
		}
	}

	result := []photoPair{}

	for _, master := range masters {
		if asset, found := assets[master.RecordName]; found {
			result = append(result, photoPair{master: master, asset: asset})
		}
	}

	return result, nil
}

func projectSharedPage(pairs []photoPair, seen map[string]bool, photoID *string, complete bool,
) ([]SharedPhoto, error) {
	result := []SharedPhoto{}

	for _, pair := range pairs {
		if seen[pair.asset.RecordName] || (photoID != nil && pair.asset.RecordName != *photoID) {
			continue
		}

		photo, err := projectSharedPhoto(pair, complete)
		if err != nil {
			return nil, err
		}

		result = append(result, photo)
		seen[pair.asset.RecordName] = true

		if photoID != nil {
			return result, nil
		}
	}

	return result, nil
}

func projectSharedPhoto(pair photoPair, complete bool) (SharedPhoto, error) {
	photo, err := sharedPhotoResources(pair.master, pair.asset)
	if err != nil {
		return SharedPhoto{}, err
	}

	for name, resource := range photo.Versions {
		var kind cloudkit.PhotoFileType

		_ = json.Unmarshal(resource.Type, &kind)

		if !photo.IsLivePhoto || kind != cloudkit.PhotoFileTypeComAppleQuicktimeMovie {
			resource.Filename = photo.Filename
			photo.Versions[name] = resource
		}
	}

	if !complete {
		return SharedPhoto{Photo: photo, LikeCount: 0, Liked: false}, nil
	}

	err = fillSharedPhoto(&photo, pair)
	if err != nil {
		return SharedPhoto{}, err
	}

	return sharedPhotoLikes(photo, pair.asset)
}

func sharedPhotoResources(master, asset cloudkit.CKRecord) (Photo, error) {
	filename, err := sharedPhotoFilename(master)
	if err != nil {
		return Photo{}, err
	}

	var photo Photo

	photo.ID, photo.Filename = asset.RecordName, filename

	photo.ItemType, err = sharedPhotoKind(master, photo.Filename)
	if err != nil {
		return Photo{}, err
	}

	photo.IsLivePhoto = false
	if photo.ItemType == Image && master.Fields != nil {
		_, photo.IsLivePhoto = (*master.Fields)[string(cloudkit.ResOriginalVidComplFileType)]
	}

	photo.Versions, err = photoResources(master, photo)
	if err != nil {
		return Photo{}, err
	}

	delete(photo.Versions, string(cloudkit.Alternative))
	delete(photo.Versions, string(cloudkit.Sidecar))

	return photo, nil
}

func sharedPhotoKind(master cloudkit.CKRecord, filename string) (PhotoItemType, error) {
	value, err := photoFieldValue(master, string(cloudkit.ItemType))
	if err != nil {
		return "", err
	}

	if len(value) == 0 {
		value, err = photoFieldValue(master, string(cloudkit.ResOriginalFileType))
		if err != nil {
			return "", err
		}
	}

	var kind cloudkit.PhotoFileType

	_ = json.Unmarshal(value, &kind)

	if kind == cloudkit.PhotoFileTypePublicHeic || kind == cloudkit.PhotoFileTypePublicJpeg ||
		kind == cloudkit.PhotoFileTypePublicPng {
		return Image, nil
	}

	if kind == cloudkit.PhotoFileTypeComAppleQuicktimeMovie {
		return Movie, nil
	}

	return sharedImageFilename(filename), nil
}

func sharedImageFilename(filename string) PhotoItemType {
	for _, extension := range []sharedphotos.SharedImageExtension{sharedphotos.DotHeic, sharedphotos.DotPng,
		sharedphotos.DotJpg, sharedphotos.DotJpeg} {
		if strings.HasSuffix(strings.ToLower(filename), string(extension)) {
			return Image
		}
	}

	return Movie
}

func fillSharedPhoto(photo *Photo, pair photoPair) error {
	photo.MasterID = pair.master.RecordName

	metadata, err := json.Marshal(pair.asset)
	if err != nil {
		return fmt.Errorf("encode shared photo metadata: %w", err)
	}

	photo.AssetMetadata = metadata

	photo.Dimensions, err = photoMasterValues(pair.master, []cloudkit.PhotoMasterField{
		cloudkit.ResOriginalWidth, cloudkit.ResOriginalHeight})
	if err != nil {
		return err
	}

	photo.Added = sharedPhotoDate(pair.asset, string(cloudkit.AddedDate))
	photo.Created = sharedPhotoDate(pair.master, string(sharedphotos.OriginalCreationDate))
	photo.Size = sharedPhotoSize(pair.master, string(sharedphotos.ResOriginalFileSize))

	return nil
}

func sharedPhotoLikes(photo Photo, record cloudkit.CKRecord) (SharedPhoto, error) {
	plugin := new(sharedphotos.SharedPluginFields)

	if record.PluginFields != nil {
		encoded, err := json.Marshal(*record.PluginFields)
		if err != nil {
			return SharedPhoto{}, fmt.Errorf("encode stream likes: %w", err)
		}

		err = json.Unmarshal(encoded, plugin)
		if err != nil {
			return SharedPhoto{}, fmt.Errorf("decode stream likes: %w", err)
		}
	}

	result := SharedPhoto{Photo: photo, LikeCount: 0, Liked: false}
	if plugin.LikeCount != nil {
		result.LikeCount = plugin.LikeCount.Value
	}

	if plugin.LikedByCaller != nil {
		result.Liked = plugin.LikedByCaller.Value
	}

	return result, nil
}
