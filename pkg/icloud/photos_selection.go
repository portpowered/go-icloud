package icloud

import (
	"errors"
	"github.com/portpowered/go-icloud/internal/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

var errPhotoLibraryIdentity = errors.New("photo library requires a zone name")

func photosRequestContext(auth AuthContext, library *PhotoLibrary) (webtransport.RequestContext, error) {
	boundary, err := accountRequestContext(auth)
	if err != nil {
		return boundary, err
	}
	boundary.Origin = auth.PhotosServiceURL
	if library == nil {
		return boundary, nil
	}
	if library.ZoneName == "" {
		return boundary, errPhotoLibraryIdentity
	}
	zone := new(cloudkit.CKZoneIDReq)
	zone.ZoneName = library.ZoneName
	if value, nullableErr := library.ZoneType.Get(); nullableErr == nil {
		zone.ZoneType.Set(value)
	}
	if value, nullableErr := library.OwnerRecordName.Get(); nullableErr == nil {
		zone.OwnerRecordName.Set(value)
	}
	boundary.PhotoZone = zone
	boundary.PhotoShared = library.Shared
	return boundary, nil
}
