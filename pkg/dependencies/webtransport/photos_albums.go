package webtransport

import (
	"context"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// PhotosAlbumQuery reads one primary-library album page in Source request order.
func (client *Client) PhotosAlbumQuery(ctx context.Context, auth RequestContext,
	parent, continuation *string,
) (*PhotosQueryResponse, error) {
	body, err := photosAlbumBody(parent, continuation, auth.PhotoZone)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	return client.photosQueryBytes(ctx, auth, body)
}

func photosAlbumBody(parent, continuation *string, selected ...*cloudkit.CKZoneIDReq) (string, error) {
	input := new(cloudkit.CKQueryRequest)
	input.Query.RecordType = protocol.PhotosPhotoAlbumQueryRecordTypeValue

	if parent != nil && *parent != "" {
		filter, err := photosStringFilter(protocol.PhotosPhotoAlbumParentFieldValue, *parent)
		if err != nil {
			return "", err
		}

		input.Query.FilterBy.Set([]cloudkit.CKQueryFilterBy{filter})
	}

	zone := photosQueryZone(selected...)
	input.ZoneID.Set(zone)
	if continuation != nil {
		input.ContinuationMarker.Set(*continuation)
	}

	return photosQueryBody(*input, zone)
}
