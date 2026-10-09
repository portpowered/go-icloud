package icloud

import (
	"context"
	"slices"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// ListRecentlyAddedPhotos enumerates the primary library newest first using trailing rank windows.
func (sdk *SDK) ListRecentlyAddedPhotos(ctx context.Context,
	request ListRecentlyAddedPhotosRequest,
) (*ListRecentlyAddedPhotosResult, error) {
	read, err := sdk.beginPhotosRead(ctx, request.Auth, "ListRecentlyAddedPhotos", request.Library)
	if err != nil {
		return nil, err
	}

	err = read.discoverPhotoLibraries(ctx)
	if err != nil {
		return nil, err
	}

	photos, err := read.recentlyAdded(ctx)
	if err != nil {
		return nil, err
	}

	return &ListRecentlyAddedPhotosResult{Photos: photos, Responses: read.metadata()}, nil
}

func (read *photosRead) recentlyAdded(ctx context.Context) ([]Photo, error) {
	photos := []Photo{}
	seen := map[string]bool{}

	offset := int64(photoSourcePageSize - 1)

	for {
		response, err := read.sdk.web.PhotosAssetQuery(ctx, read.auth, cloudkit.CPLAssetAndMasterByAddedDate,
			cloudkit.DESCENDING, offset, nil)
		if err != nil {
			return nil, read.failure(err, InvalidResponse)
		}

		read.responses = append(read.responses, response.Metadata)

		records, err := photoNormalRecords(response.Data)
		if err != nil {
			return nil, read.failure(err, InvalidResponse)
		}

		pairs, err := projectPhotoPairs(records)
		if err != nil {
			return nil, read.failure(err, InvalidResponse)
		}

		window, err := projectSelectedPhotoPage(pairs, seen, nil, projectPhoto)
		if err != nil {
			return nil, read.failure(err, InvalidResponse)
		}

		slices.Reverse(window)

		photos = append(photos, window...)
		if len(window) < photoSourcePageSize {
			return photos, nil
		}

		offset += int64(len(window))
	}
}
