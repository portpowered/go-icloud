package webtransport

import "github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"

// ExportReferenceJSONEscapes exposes the private byte transducer only to external tests.
func ExportReferenceJSONEscapes(encoded []byte) []byte {
	return referenceJSONEscapes(encoded)
}

// ExportPhotosAssetBodyLimit exposes generated query serialization only to external tests.
func ExportPhotosAssetBodyLimit(index cloudkit.PhotoListIndex, direction cloudkit.PhotoDirection, offset int64,
	extra []PhotosAssetSelector, limit int64,
) (string, error) {
	return photosAssetBodyLimit(index, direction, offset, extra, limit)
}

// ExportPhotosAlbumBody exposes generated album serialization only to external tests.
func ExportPhotosAlbumBody(parent, continuation *string, zone *cloudkit.CKZoneIDReq) (string, error) {
	return photosAlbumBody(parent, continuation, zone)
}
