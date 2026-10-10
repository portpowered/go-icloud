package webtransport

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const photosAssetResultsLimit = 200

// PhotosAssetSelector supplies an additional string equality selector.
type PhotosAssetSelector struct {
	// Field is a schema-owned query field.
	Field cloudkit.PhotoAssetQueryField
	// Value is the selected album or asset identifier.
	Value string
}

// PhotosAssetQuery reads a paired asset/master page with Source filter order.
func (client *Client) PhotosAssetQuery(ctx context.Context, auth RequestContext,
	index cloudkit.PhotoListIndex, direction cloudkit.PhotoDirection, offset int64,
	extra []PhotosAssetSelector,
) (*PhotosQueryResponse, error) {
	body, err := photosAssetBody(index, direction, offset, extra, auth.PhotoZone)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	return client.photosQueryBytes(ctx, auth, body)
}

func photosAssetBody(index cloudkit.PhotoListIndex, direction cloudkit.PhotoDirection, offset int64,
	extra []PhotosAssetSelector, selected ...*cloudkit.CKZoneIDReq,
) (string, error) {
	return photosAssetBodyLimit(index, direction, offset, extra, photosAssetResultsLimit, selected...)
}

func photosAssetBodyLimit(index cloudkit.PhotoListIndex, direction cloudkit.PhotoDirection, offset int64,
	extra []PhotosAssetSelector, limit int64, selected ...*cloudkit.CKZoneIDReq,
) (string, error) {
	filters, err := photosAssetFilters(direction, offset, extra)
	if err != nil {
		return "", err
	}

	zone := photosQueryZone(selected...)
	input := new(cloudkit.CKQueryRequest)
	input.Query.RecordType = string(index)
	input.Query.FilterBy.Set(filters)
	input.ZoneID.Set(zone)
	input.ResultsLimit.Set(limit)

	return photosQueryBody(input, zone)
}

func photosAssetFilters(direction cloudkit.PhotoDirection, offset int64,
	extra []PhotosAssetSelector,
) ([]cloudkit.CKQueryFilterBy, error) {
	directionFilter, err := photosStringFilter(string(cloudkit.PhotoAssetQueryFieldDirection), string(direction))
	if err != nil {
		return nil, err
	}

	rankFilter, err := photosRankFilter(offset)
	if err != nil {
		return nil, err
	}

	filters := []cloudkit.CKQueryFilterBy{directionFilter, rankFilter}

	for _, selector := range extra {
		filter, filterErr := photosStringFilter(string(selector.Field), selector.Value)
		if filterErr != nil {
			return nil, filterErr
		}

		filters = append(filters, filter)
	}

	return filters, nil
}
