package icloud

import (
	"context"
	"errors"
)

// PhotoVisitor receives owned photo projections synchronously. False stops iteration.
type PhotoVisitor func(PhotoVisitEvent) (bool, error)

var errPhotoVisitStopped = errors.New("photo visitor stopped")

// VisitPhotoAssets visits album assets incrementally and preserves consumed responses.
func (sdk *SDK) VisitPhotoAssets(
	ctx context.Context,
	request ListPhotoAssetsRequest,
	visitor PhotoVisitor,
) (*ListPhotoAssetsResult, error) {
	return sdk.listPhotoAssets(ctx, request, visitor)
}

// VisitRecentlyAddedPhotos visits newest-first assets, stopping before another rank window.
func (sdk *SDK) VisitRecentlyAddedPhotos(
	ctx context.Context,
	request ListRecentlyAddedPhotosRequest,
	visitor PhotoVisitor,
) (*ListRecentlyAddedPhotosResult, error) {
	return sdk.listRecentlyAddedPhotos(ctx, request, visitor)
}
func (read *photosRead) photoVisitor() func(Photo) (bool, error) {
	if read.visitor == nil {
		return nil
	}

	return func(photo Photo) (bool, error) {
		return read.visitor(PhotoVisitEvent{Photo: photo, Responses: read.metadata()})
	}
}
