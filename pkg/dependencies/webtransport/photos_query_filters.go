package webtransport

import (
	"fmt"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func photosStringFilter(name, value string) (cloudkit.CKQueryFilterBy, error) {
	field := new(cloudkit.CKQueryFilterBy_FieldValue)

	err := field.FromCKFVString(cloudkit.CKFVString{Type: cloudkit.CKFVStringTypeSTRING,
		Value: value, AdditionalProperties: nil})
	if err != nil {
		var empty cloudkit.CKQueryFilterBy

		return empty, fmt.Errorf("encode photo string selector: %w", err)
	}

	return photosQueryFilter(name, *field)
}

func photosRankFilter(offset int64) (cloudkit.CKQueryFilterBy, error) {
	field := new(cloudkit.CKQueryFilterBy_FieldValue)

	err := field.FromCKFVInt64(cloudkit.CKFVInt64{Type: cloudkit.CKFVInt64TypeINT64,
		Value: max(offset, 0), AdditionalProperties: nil})
	if err != nil {
		var empty cloudkit.CKQueryFilterBy

		return empty, fmt.Errorf("encode photo rank selector: %w", err)
	}

	return photosQueryFilter(string(cloudkit.PhotoAssetQueryFieldStartRank), *field)
}

func photosQueryFilter(name string, field cloudkit.CKQueryFilterBy_FieldValue) (cloudkit.CKQueryFilterBy, error) {
	filter := new(cloudkit.CKQueryFilterBy)
	filter.FieldName = name
	filter.FieldValue = field

	err := filter.Comparator.FromCKComparator(cloudkit.CKComparatorEQUALS)
	if err != nil {
		var empty cloudkit.CKQueryFilterBy

		return empty, fmt.Errorf("encode photo equality selector: %w", err)
	}

	return *filter, nil
}
