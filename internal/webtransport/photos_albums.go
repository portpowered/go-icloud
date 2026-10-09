package webtransport

import (
	"context"
	"fmt"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// PhotosAlbumQuery reads one primary-library album page in Source request order.
func (client *Client) PhotosAlbumQuery(ctx context.Context, auth RequestContext,
	parent, continuation *string,
) (*PhotosQueryResponse, error) {
	body, err := photosAlbumBody(parent, continuation)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	return client.photosQueryBytes(ctx, auth, body)
}

func photosAlbumBody(parent, continuation *string) (string, error) {
	query := fmt.Sprintf("{%q: %q", protocol.PhotosCKQueryObjectRecordType, protocol.PhotosPhotoAlbumQueryRecordTypeValue)

	if parent != nil && *parent != "" {
		name, err := referenceJSON(*parent)
		if err != nil {
			return "", err
		}

		value := fmt.Sprintf("{%q: %q, %q: %s}", protocol.PhotosCKFVStringType, cloudkit.CKFVStringTypeSTRING,
			protocol.PhotosCKFVStringValue, name)
		filter := fmt.Sprintf("{%q: %q, %q: %q, %q: %s}", protocol.PhotosCKQueryFilterByComparator,
			cloudkit.EQUALS, protocol.PhotosCKQueryFilterByFieldName, protocol.PhotosPhotoAlbumParentFieldValue,
			protocol.PhotosCKQueryFilterByFieldValue, value)
		query += fmt.Sprintf(", %q: [%s]", protocol.PhotosCKQueryObjectFilterBy, filter)
	}

	query += "}"
	zone := cloudkit.CKZoneIDReq{ZoneName: protocol.PhotosPhotoPrimaryZoneNameValue,
		ZoneType: nil, OwnerRecordName: nil, AdditionalProperties: nil}
	zone.ZoneType.Set(protocol.PhotosPhotoPrimaryZoneTypeValue)

	identity, err := referenceJSON(zone)
	if err != nil {
		return "", err
	}

	body := fmt.Sprintf("{%q: %s, %q: %s", protocol.PhotosCKQueryRequestQuery, query,
		protocol.PhotosCKQueryRequestZoneID, identity)

	if continuation != nil {
		token, err := referenceJSON(*continuation)
		if err != nil {
			return "", err
		}

		body += fmt.Sprintf(", %q: %s", protocol.PhotosCKQueryRequestContinuationMarker, token)
	}

	return body + "}", nil
}
