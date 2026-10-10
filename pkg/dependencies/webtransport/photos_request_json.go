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
