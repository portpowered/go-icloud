package webtransport

import (
	"bytes"
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// photosOrderedZone preserves Source identity order inside an encoded generated request.
func photosOrderedZone(body []byte, zone cloudkit.CKZoneIDReq) ([]byte, error) {
	original, err := referenceJSON(zone)
	if err != nil {
		return nil, fmt.Errorf("encode photo request zone: %w", err)
	}

	ordered, err := referenceJSONFields(zone, []string{protocol.PhotosCKZoneIDReqZoneName,
		protocol.PhotosCKZoneIDReqZoneType, protocol.PhotosCKZoneIDReqOwnerRecordName})
	if err != nil {
		return nil, fmt.Errorf("order photo request zone: %w", err)
	}

	return bytes.ReplaceAll(body, original, ordered), nil
}

func photosQueryZone(selected ...*cloudkit.CKZoneIDReq) cloudkit.CKZoneIDReq {
	zone := new(cloudkit.CKZoneIDReq)
	zone.ZoneName = protocol.PhotosPhotoPrimaryZoneNameValue
	zone.ZoneType.Set(protocol.PhotosPhotoPrimaryZoneTypeValue)
	if len(selected) > 0 && selected[0] != nil {
		zone = selected[0]
	}

	return *zone
}

func photosQueryBody(input *cloudkit.CKQueryRequest, zone cloudkit.CKZoneIDReq) (string, error) {
	body, err := referenceJSONFields(input, []string{protocol.PhotosCKQueryRequestQuery,
		protocol.PhotosCKQueryRequestZoneID, protocol.PhotosCKQueryRequestResultsLimit,
		protocol.PhotosCKQueryRequestContinuationMarker})
	if err != nil {
		return "", fmt.Errorf("encode photo query request: %w", err)
	}

	body, err = photosOrderedQuery(body, input.Query)
	if err != nil {
		return "", err
	}

	body, err = photosOrderedZone(body, zone)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func photosOrderedQuery(body []byte, query cloudkit.CKQueryObject) ([]byte, error) {
	original, err := referenceJSON(query)
	if err != nil {
		return nil, fmt.Errorf("encode photo query object: %w", err)
	}

	ordered, err := referenceJSONFields(query, []string{protocol.PhotosCKQueryObjectRecordType,
		protocol.PhotosCKQueryObjectFilterBy, protocol.PhotosCKQueryObjectSortBy})
	if err != nil {
		return nil, fmt.Errorf("order photo query object: %w", err)
	}

	return bytes.ReplaceAll(body, original, ordered), nil
}
