package webtransport

import (
	"context"
	"fmt"
	"strings"

	"github.com/portpowered/go-icloud/internal/protocol"
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
	body, err := photosAssetBody(index, direction, offset, extra)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	return client.photosQueryBytes(ctx, auth, body)
}

func photosAssetBody(index cloudkit.PhotoListIndex, direction cloudkit.PhotoDirection, offset int64,
	extra []PhotosAssetSelector,
) (string, error) {
	return photosAssetBodyLimit(index, direction, offset, extra, photosAssetResultsLimit)
}

func photosAssetBodyLimit(index cloudkit.PhotoListIndex, direction cloudkit.PhotoDirection, offset int64,
	extra []PhotosAssetSelector, limit int64,
) (string, error) {
	filters := []string{}

	for _, selector := range append([]PhotosAssetSelector{{Field: cloudkit.PhotoAssetQueryFieldDirection,
		Value: string(direction)}}, extra...) {
		value := cloudkit.CKFVString{Type: cloudkit.CKFVStringTypeSTRING, Value: selector.Value, AdditionalProperties: nil}

		filter, err := photosAssetFilter(selector.Field, value)
		if err != nil {
			return "", err
		}

		filters = append(filters, filter)
	}

	rank, err := photosAssetFilter(cloudkit.PhotoAssetQueryFieldStartRank,
		cloudkit.CKFVInt64{Type: cloudkit.CKFVInt64TypeINT64, Value: max(offset, 0), AdditionalProperties: nil})
	if err != nil {
		return "", err
	}

	filters = append(filters[:1], append([]string{rank}, filters[1:]...)...)
	zone := cloudkit.CKZoneIDReq{ZoneName: protocol.PhotosPhotoPrimaryZoneNameValue,
		ZoneType: nil, OwnerRecordName: nil, AdditionalProperties: nil}
	zone.ZoneType.Set(protocol.PhotosPhotoPrimaryZoneTypeValue)

	identity, err := referenceJSON(zone)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("{%q: {%q: %q, %q: [%s]}, %q: %s, %q: %d}",
		protocol.PhotosCKQueryRequestQuery, protocol.PhotosCKQueryObjectRecordType, index,
		protocol.PhotosCKQueryObjectFilterBy, strings.Join(filters, ", "),
		protocol.PhotosCKQueryRequestZoneID, identity, protocol.PhotosCKQueryRequestResultsLimit,
		limit), nil
}

func photosAssetFilter(name cloudkit.PhotoAssetQueryField, field any) (string, error) {
	value, err := referenceJSON(field)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("{%q: %q, %q: %q, %q: %s}",
		protocol.PhotosCKQueryFilterByComparator, cloudkit.CKComparatorEQUALS,
		protocol.PhotosCKQueryFilterByFieldName, name, protocol.PhotosCKQueryFilterByFieldValue, value), nil
}
