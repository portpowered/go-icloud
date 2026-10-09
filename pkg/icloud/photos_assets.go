package icloud

import (
	"context"
	"strings"

	"github.com/portpowered/go-icloud/internal/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const photoSourcePageSize = 100

// ListPhotoAssets enumerates the selected primary album with Source ordering and deduplication.
func (sdk *SDK) ListPhotoAssets(ctx context.Context, request ListPhotoAssetsRequest) (*ListPhotoAssetsResult, error) {
	read, err := sdk.beginPhotosRead(ctx, request.Auth, "ListPhotoAssets")
	if err != nil {
		return nil, err
	}

	albums, err := read.albums(ctx, nil)
	if err != nil {
		return nil, err
	}

	entries, err := projectPhotoAlbums(albums)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	entry, found := findPhotoAlbumEntry(entries, request.Album)
	if !found {
		return nil, read.failure(errPhotoAlbumMissing, NotFound)
	}

	photos, err := read.assets(ctx, entry)
	if err != nil {
		return nil, err
	}

	return &ListPhotoAssetsResult{Photos: photos, Responses: read.metadata()}, nil
}

func findPhotoAlbumEntry(entries []photoAlbumEntry, selector string) (photoAlbumEntry, bool) {
	for _, entry := range entries {
		if entry.album.ID == selector {
			return entry, true
		}
	}

	for _, entry := range entries {
		if entry.album.Name == selector || entry.album.FullName == selector {
			return entry, true
		}
	}

	var absent photoAlbumEntry

	return absent, false
}

func photoQuerySpec(entry photoAlbumEntry) photoAlbumQuerySpec {
	if strings.HasPrefix(entry.index, string(cloudkit.CPLContainerRelationNotDeletedByAssetDate)+":") {
		direction := cloudkit.ASCENDING
		if entry.descending {
			direction = cloudkit.DESCENDING
		}

		return photoAlbumQuerySpec{index: cloudkit.CPLContainerRelationLiveByAssetDate, direction: direction,
			filters: []webtransport.PhotosAssetSelector{{Field: cloudkit.PhotoAssetQueryFieldParentId, Value: entry.album.ID}}}
	}

	return photoSmartQueries()[cloudkit.PhotoSmartAlbumName(entry.album.ID)]
}

func (read *photosRead) assets(ctx context.Context, entry photoAlbumEntry) ([]Photo, error) {
	spec := photoQuerySpec(entry)
	offset := int64(0)

	if spec.direction == cloudkit.DESCENDING {
		response, err := read.sdk.web.PhotosAlbumCount(ctx, read.auth, entry.index)
		if err != nil {
			return nil, read.failure(err, InvalidResponse)
		}

		read.responses = append(read.responses, response.Metadata)

		count, err := photoCount(response.Data)
		if err != nil {
			return nil, read.failure(err, InvalidResponse)
		}

		offset = count - 1
	}

	return read.assetPages(ctx, spec, offset)
}

func (read *photosRead) assetPages(ctx context.Context, spec photoAlbumQuerySpec, offset int64) ([]Photo, error) {
	photos := []Photo{}
	seen := map[string]bool{}

	for {
		response, err := read.sdk.web.PhotosAssetQuery(ctx, read.auth, spec.index, spec.direction, offset, spec.filters)
		if err != nil {
			return nil, read.failure(err, InvalidResponse)
		}

		read.responses = append(read.responses, response.Metadata)

		records, err := photoNormalRecords(response.Data)
		if err != nil {
			return nil, read.failure(err, InvalidResponse)
		}

		page, err := projectPhotoPairs(records)
		if err != nil {
			return nil, read.failure(err, InvalidResponse)
		}

		for _, pair := range page {
			if !seen[pair.asset.RecordName] {
				photo, err := projectPhoto(pair.master, pair.asset)
				if err != nil {
					return nil, read.failure(err, InvalidResponse)
				}

				seen[photo.ID] = true

				photos = append(photos, photo)
			}
		}

		if len(page) < photoSourcePageSize/2 {
			return photos, nil
		}

		if spec.direction == cloudkit.DESCENDING {
			offset -= int64(len(page))
		} else {
			offset += int64(len(page))
		}
	}
}

type photoPair struct {
	master cloudkit.CKRecord
	asset  cloudkit.CKRecord
}

func projectPhotoPairs(records []cloudkit.CKRecord) ([]photoPair, error) {
	assets := map[string]cloudkit.CKRecord{}
	masters := []cloudkit.CKRecord{}

	for _, record := range records {
		switch cloudkit.PhotoRecordKind(record.RecordType) {
		case cloudkit.CPLAsset:
			master, err := reminderReference(record, string(cloudkit.MasterRef))
			if err != nil {
				return nil, err
			}

			if master == "" {
				master = record.RecordName
			}

			assets[master] = record
		case cloudkit.CPLMaster:
			masters = append(masters, record)
		}
	}

	photos := []photoPair{}

	for _, master := range masters {
		asset, found := assets[master.RecordName]
		if !found {
			continue
		}

		photos = append(photos, photoPair{master: master, asset: asset})
	}

	return photos, nil
}
