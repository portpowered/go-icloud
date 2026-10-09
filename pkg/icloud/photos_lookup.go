package icloud

import (
	"context"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// GetPhoto checks the direct album lookup, then scans album pages until the asset is found.
// A photo absent from both sources is returned as explicit null with complete response evidence.
func (sdk *SDK) GetPhoto(ctx context.Context, request GetPhotoRequest) (*GetPhotoResult, error) {
	read, err := sdk.beginPhotosRead(ctx, request.Auth, "GetPhoto")
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

	photos, err := read.lookupPhoto(ctx, entry, request.PhotoID)
	if err != nil {
		return nil, err
	}

	result := &GetPhotoResult{Photo: nil, Responses: read.metadata()}
	result.Photo.SetNull()

	if len(photos) != 0 {
		result.Photo.Set(photos[0])
	}

	return result, nil
}

func (read *photosRead) lookupPhoto(ctx context.Context, entry photoAlbumEntry, photoID string) ([]Photo, error) {
	spec := photoQuerySpec(entry)

	response, err := read.sdk.web.PhotosLookupAsset(ctx, read.auth, spec.index, photoID, spec.filters)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, response.Metadata)

	photos, err := selectPhotoPair(response.Data, photoID)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	if len(photos) != 0 {
		return photos, nil
	}

	offset, err := read.assetOffset(ctx, entry, spec)
	if err != nil {
		return nil, err
	}

	return read.assetPages(ctx, spec, offset, &photoID)
}

func selectPhotoPair(data cloudkit.CKQueryResponse, photoID string) ([]Photo, error) {
	records, err := photoNormalRecords(data)
	if err != nil {
		return nil, err
	}

	pairs, err := projectPhotoPairs(records)
	if err != nil {
		return nil, err
	}

	for _, pair := range pairs {
		if pair.asset.RecordName == photoID {
			photo, err := projectPhoto(pair.master, pair.asset)
			if err != nil {
				return nil, err
			}

			return []Photo{photo}, nil
		}
	}

	return []Photo{}, nil
}
