package icloud

import (
	"errors"

	"github.com/portpowered/go-icloud/internal/protocol"
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

	value, nullableErr := library.ZoneType.Get()
	if nullableErr == nil {
		zone.ZoneType.Set(value)
	}

	value, nullableErr = library.OwnerRecordName.Get()
	if nullableErr == nil {
		zone.OwnerRecordName.Set(value)
	}

	boundary.PhotoZone = zone
	boundary.PhotoShared = library.Shared

	return boundary, nil
}

func (read *photosRead) projectAlbums(records []cloudkit.CKRecord) ([]photoAlbumEntry, error) {
	entries, err := projectPhotoAlbums(records)
	if err != nil || !read.sharedLibrary {
		return entries, err
	}

	selected := []photoAlbumEntry{}

	for _, entry := range entries {
		if entry.album.ID == string(cloudkit.Library) || entry.album.ID == string(cloudkit.Favorites) {
			entry.descending = true
			selected = append(selected, entry)
		}
	}

	return selected, nil
}

func (read *photosRead) querySpec(entry photoAlbumEntry) photoAlbumQuerySpec {
	spec := photoQuerySpec(entry)
	if read.sharedLibrary {
		spec.direction = cloudkit.DESCENDING
	}

	return spec
}

func photoLibraryInitialized(library *PhotoLibrary) bool {
	return library != nil && string(library.IndexingState) == protocol.PhotosPhotoFinishedStateValue
}
