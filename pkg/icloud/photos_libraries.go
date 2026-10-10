package icloud

import (
	"context"
	"errors"
	"strings"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// ListPhotoLibraries discovers initialized private and shared databases with caller-owned cursors.
func (sdk *SDK) ListPhotoLibraries(
	ctx context.Context,
	request ListPhotoLibrariesRequest,
) (*ListPhotoLibrariesResult, error) {
	read, err := sdk.beginPhotosRead(ctx, request.Auth, "ListPhotoLibraries")
	if err != nil {
		return nil, err
	}

	root := PhotoLibrary{ID: "root", ZoneName: protocol.PhotosPhotoPrimaryZoneNameValue, Shared: false,
		IsSharedLibrary: false, IndexingState: PhotoLibraryIndexingStateFINISHED,
		ZoneType: nil, OwnerRecordName: nil, SyncToken: nil}
	root.ZoneType.Set(protocol.PhotosPhotoPrimaryZoneTypeValue)
	root.OwnerRecordName.SetNull()
	root.SyncToken.SetNull()
	if read.syncToken != nil {
		root.SyncToken.Set(*read.syncToken)
	}

	read.libraries = append(read.libraries, root)

	err = read.discoverPhotoLibraries(ctx)
	if err != nil {
		return nil, err
	}

	return &ListPhotoLibrariesResult{Libraries: read.libraries, Responses: read.metadata()}, nil
}

func (read *photosRead) discoverPhotoLibraries(ctx context.Context) error {
	seen := map[string]bool{}

	err := read.initializePhotoScope(ctx, false, seen)
	if err != nil {
		return err
	}

	err = read.initializePhotoScope(ctx, true, seen)
	if err == nil {
		return nil
	}

	response := webtransport.SuppressedPhotosResponse(err)
	if response != nil {
		read.responses = append(read.responses, response)

		return nil
	}

	var failure *ClientError
	if errors.As(err, &failure) && (failure.Kind() == Unavailable || failure.Kind() == InvalidResponse) {
		return nil
	}

	return err
}

func (read *photosRead) initializePhotoScope(ctx context.Context, shared bool, seen map[string]bool) error {
	page, err := read.sdk.web.PhotosLibraryZones(ctx, read.auth, shared)
	if err != nil {
		return read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, page.Metadata)

	if page.Data.Zones == nil {
		return nil
	}

	for _, zone := range *page.Data.Zones {
		if read.skipLibraryZone(zone, shared) {
			continue
		}

		key := photoLibraryKey(zone.ZoneID.ZoneName, shared)
		if shared && seen[key] {
			continue
		}

		err = read.initializePhotoLibrary(ctx, zone.ZoneID, shared)
		if err != nil {
			return err
		}

		seen[key] = true
	}

	return nil
}

func photoLibraryKey(name string, shared bool) string {
	if shared || strings.HasPrefix(name, protocol.PhotosPhotoSharedLibraryZonePrefixValue) {
		return protocol.PhotosPhotoSharedLibraryKeyPrefixValue + name
	}

	return name
}

func (read *photosRead) initializePhotoLibrary(ctx context.Context, zone cloudkit.CKZoneID, shared bool) error {
	response, err := read.sdk.web.PhotosLibraryIndexing(ctx, read.auth, zone, shared)
	if err != nil {
		return read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, response.Metadata)

	state, err := photosIndexingState(response.Data)
	if err != nil {
		return read.failure(err, InvalidResponse)
	}

	if state != protocol.PhotosPhotoFinishedStateValue {
		return read.failure(errPhotosIndexing, Unavailable)
	}

	library := PhotoLibrary{ID: photoLibraryKey(zone.ZoneName, shared), ZoneName: zone.ZoneName,
		Shared:          shared,
		IsSharedLibrary: shared || strings.HasPrefix(zone.ZoneName, protocol.PhotosPhotoSharedLibraryZonePrefixValue),
		IndexingState:   PhotoLibraryIndexingStateFINISHED, ZoneType: zone.ZoneType,
		OwnerRecordName: zone.OwnerRecordName, SyncToken: response.Data.SyncToken}
	if !library.ZoneType.IsSpecified() {
		library.ZoneType.SetNull()
	}

	if !library.OwnerRecordName.IsSpecified() {
		library.OwnerRecordName.SetNull()
	}

	if !library.SyncToken.IsSpecified() {
		library.SyncToken.SetNull()
	}

	for index, current := range read.libraries {
		if current.ID == library.ID {
			read.libraries[index] = library

			return nil
		}
	}

	read.libraries = append(read.libraries, library)

	return nil
}

func (read *photosRead) skipLibraryZone(zone cloudkit.CKZoneListZone, shared bool) bool {
	deleted, _ := zone.Deleted.Get()
	if deleted {
		return true
	}

	if shared || zone.ZoneID.ZoneName != protocol.PhotosPhotoPrimaryZoneNameValue {
		return false
	}

	if len(read.libraries) != 0 {
		read.libraries[0].SyncToken = zone.SyncToken
		if !read.libraries[0].SyncToken.IsSpecified() {
			read.libraries[0].SyncToken.SetNull()
		}
	}

	return true
}
