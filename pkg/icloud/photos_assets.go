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

	offset, err := read.assetOffset(ctx, entry, spec)
	if err != nil {
		return nil, err
	}

	return read.assetPages(ctx, spec, offset, nil, projectPhoto)
}

func (read *photosRead) assetOffset(ctx context.Context, entry photoAlbumEntry,
	spec photoAlbumQuerySpec,
) (int64, error) {
	offset := int64(0)

	if spec.direction == cloudkit.DESCENDING {
		response, err := read.sdk.web.PhotosAlbumCount(ctx, read.auth, entry.index)
		if err != nil {
			return 0, read.failure(err, InvalidResponse)
		}

		read.responses = append(read.responses, response.Metadata)

		count, err := photoCount(response.Data)
		if err != nil {
			return 0, read.failure(err, InvalidResponse)
		}

		offset = count - 1
	}

	return offset, nil
}

func (read *photosRead) assetPages(ctx context.Context, spec photoAlbumQuerySpec, offset int64,
	photoID *string,
	project photoProjection,
) ([]Photo, error) {
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

		projected, err := projectSelectedPhotoPage(page, seen, photoID, project)
		if err != nil {
			return nil, read.failure(err, InvalidResponse)
		}

		photos = append(photos, projected...)
		if photoID != nil && len(projected) != 0 {
			return photos, nil
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

func projectSelectedPhotoPage(page []photoPair, seen map[string]bool, photoID *string,
	project photoProjection,
) ([]Photo, error) {
	photos := []Photo{}

	for _, pair := range page {
		if seen[pair.asset.RecordName] || (photoID != nil && pair.asset.RecordName != *photoID) {
			continue
		}

		photo, err := project(pair.master, pair.asset)
		if err != nil {
			return nil, err
		}

		seen[photo.ID] = true

		photos = append(photos, photo)
		if photoID != nil {
			return photos, nil
		}
	}

	return photos, nil
}
