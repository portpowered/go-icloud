package webtransport

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const (
	photosLookupResultsLimit          = 2
	photosContainerLookupResultsLimit = 3
)

// PhotosLookupAsset queries one asset in the selected primary album index.
func (client *Client) PhotosLookupAsset(ctx context.Context, auth RequestContext,
	index cloudkit.PhotoListIndex, photoID string, extra []PhotosAssetSelector,
) (*PhotosQueryResponse, error) {
	limit := int64(photosLookupResultsLimit)
	if index == cloudkit.CPLContainerRelationLiveByAssetDate {
		limit = photosContainerLookupResultsLimit
	}

	filters := append([]PhotosAssetSelector{{Field: cloudkit.PhotoAssetQueryFieldRecordName, Value: photoID}}, extra...)

	body, err := photosAssetBodyLimit(index, cloudkit.ASCENDING, 0, filters, limit, auth.PhotoZone)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	return client.photosQueryBytes(ctx, auth, body)
}
