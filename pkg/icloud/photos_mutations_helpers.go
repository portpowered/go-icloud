package icloud

import (
	"context"
	"errors"
	"fmt"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/internal/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	pm "github.com/portpowered/go-icloud/pkg/dependencymodels/photosmutations"
)

var errPhotoMutationRejected = errors.New("photo mutation rejected")

func photoMutationInteger(value int64) pm.PhotoMutationInteger {
	return pm.PhotoMutationInteger{Type: pm.INT64, Value: value}
}
func photoMutationString(value string) pm.PhotoMutationText {
	return pm.PhotoMutationText{Type: pm.STRING, Value: value}
}
func photoMutationZone(auth webtransport.RequestContext) pm.PhotoMutationZone {
	zone := pm.PhotoMutationZone{ZoneName: protocol.PhotosPhotoPrimaryZoneNameValue,
		ZoneType:        nil,
		OwnerRecordName: nil}
	kind := protocol.PhotosPhotoPrimaryZoneTypeValue

	zone.ZoneType = &kind
	if auth.PhotoZone != nil {
		zone.ZoneName = auth.PhotoZone.ZoneName

		zone.ZoneType = nil

		if auth.PhotoZone.ZoneType.IsSpecified() && !auth.PhotoZone.ZoneType.IsNull() {
			v := auth.PhotoZone.ZoneType.GetOrEmpty()
			zone.ZoneType = &v
		}

		if auth.PhotoZone.OwnerRecordName.IsSpecified() && !auth.PhotoZone.OwnerRecordName.IsNull() {
			v := auth.PhotoZone.OwnerRecordName.GetOrEmpty()
			zone.OwnerRecordName = &v
		}
	}

	return zone
}
func photoMutationTag(record cloudkit.CKRecord) *string {
	if !record.RecordChangeTag.IsSpecified() || record.RecordChangeTag.IsNull() {
		return nil
	}

	value := record.RecordChangeTag.GetOrEmpty()

	return &value
}
func (read *photosRead) modifyPhoto(ctx context.Context,
	input pm.PhotoMutationRequest, private ...bool,
) ([]cloudkit.CKRecord, bool, error) {
	auth := read.auth
	if len(private) != 0 && private[0] {
		auth.PhotoShared = false
	}
	response, err := read.sdk.web.ModifyPhotos(ctx, auth, input)
	if err != nil {
		return nil, false, read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, response.Metadata)
	records := []cloudkit.CKRecord{}

	rejected := false

	if response.Data.Records == nil {
		return records, rejected, nil
	}

	for _, item := range *response.Data.Records {
		selected, err := webtransport.DecodeReminderModificationRecord(item)
		if err != nil {
			return nil, false, read.failure(err, InvalidResponse)
		}

		if selected.Failure != nil {
			rejected = true
		}

		if selected.Record != nil {
			records = append(records, *selected.Record)
		}
	}

	return records, rejected, nil
}
func (read *photosRead) mutationAlbum(ctx context.Context, photoID string) (photoAlbumEntry, error) {
	records, err := read.albums(ctx, nil)
	if err != nil {
		return photoAlbumEntry{}, err
	}

	entries, err := projectPhotoAlbums(records)
	if err != nil {
		return photoAlbumEntry{}, read.failure(err, InvalidResponse)
	}

	for _, entry := range entries {
		if entry.album.ID == photoID {
			return entry, nil
		}
	}

	return photoAlbumEntry{}, read.failure(errPhotoAlbumMissing, NotFound)
}
func (read *photosRead) mutationPhoto(ctx context.Context, album *string,
	photoID string,
) (cloudkit.CKRecord, cloudkit.CKRecord, error) {
	name := string(cloudkit.Library)
	if album != nil {
		name = *album
	}
	entry, err := read.mutationAlbum(ctx, name)
	if err != nil {
		return cloudkit.CKRecord{}, cloudkit.CKRecord{}, err
	}
	master, asset, present, err := read.lookupMutationPhoto(ctx, entry, photoID)
	if err != nil {
		return master, asset, err
	}
	if !present {
		return master, asset, read.failure(fmt.Errorf("%w: %s", errPhotoMutationRejected, photoID), NotFound)
	}
	return master, asset, nil
}
func (read *photosRead) lookupMutationPhoto(ctx context.Context, entry photoAlbumEntry,
	photoID string,
) (cloudkit.CKRecord, cloudkit.CKRecord, bool, error) {
	var master, asset cloudkit.CKRecord
	photos, err := read.lookupPhoto(ctx, entry, photoID, func(masterRecord, assetRecord cloudkit.CKRecord) (Photo, error) {
		master = masterRecord
		asset = assetRecord
		return projectPhoto(masterRecord, assetRecord)
	})
	return master, asset, len(photos) != 0, err
}

func photoRecordMutationZone(auth webtransport.RequestContext, record cloudkit.CKRecord) pm.PhotoMutationZone {
	zone := photoMutationZone(auth)

	if record.ZoneID.IsSpecified() && !record.ZoneID.IsNull() {
		source := record.ZoneID.GetOrEmpty()
		zone.ZoneName = source.ZoneName
		zone.ZoneType = nil
		zone.OwnerRecordName = nil

		if source.ZoneType.IsSpecified() && !source.ZoneType.IsNull() {
			v := source.ZoneType.GetOrEmpty()
			zone.ZoneType = &v
		}

		if source.OwnerRecordName.IsSpecified() && !source.OwnerRecordName.IsNull() {
			v := source.OwnerRecordName.GetOrEmpty()
			zone.OwnerRecordName = &v
		}
	}

	return zone
}

func photoAlbumMutationTag(album PhotoAlbum) *string {
	if !album.RecordChangeTag.IsSpecified() || album.RecordChangeTag.IsNull() {
		return nil
	}

	value := album.RecordChangeTag.GetOrEmpty()

	return &value
}

func (sdk *SDK) beginPhotosMutation(ctx context.Context, auth AuthContext, operation string,
	library *PhotoLibrary,
) (*photosRead, error) {
	err := ctx.Err()
	if err != nil {
		return nil, driveContextFailure(operation, err)
	}
	return sdk.beginPhotosRead(ctx, auth, operation, library)
}
