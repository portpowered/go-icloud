package icloud

import (
	"context"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	pm "github.com/portpowered/go-icloud/pkg/dependencymodels/photosmutations"
)

// AddPhotoToAlbum adds a looked-up asset to a custom album as one atomic relation creation.
func (sdk *SDK) AddPhotoToAlbum(ctx context.Context,
	request AddPhotoToAlbumRequest) (*PhotoAlbumRelationResult, error) {
	read, err := sdk.beginPhotosMutation(ctx, request.Auth, "AddPhotoToAlbum", request.Library)
	if err != nil {
		return nil, err
	}

	_, asset, err := read.mutationPhoto(ctx, &request.AlbumID, request.PhotoID)
	if err != nil {
		return nil, err
	}

	return read.addPhotoRelation(ctx, request.AlbumID, asset.RecordName)
}
func (read *photosRead) addPhotoToAlbum(ctx context.Context, album PhotoAlbum, photo Photo) (bool, error) {
	result, err := read.addPhotoRelation(ctx, album.ID, photo.ID)
	if err != nil {
		return false, err
	}
	return result.Added, nil
}
func (read *photosRead) addPhotoRelation(ctx context.Context, albumID, item string) (*PhotoAlbumRelationResult, error) {
	fields := pm.PhotoAlbumRelationFields{ItemId: photoMutationString(item),
		Position:    photoMutationInteger(int64(pm.RelationPosition)),
		ContainerId: photoMutationString(albumID)}
	input := pm.PhotoAlbumRelationRequest{Operations: []pm.PhotoAlbumRelationOperation{{
		OperationType: pm.PhotoAlbumRelationOperationOperationTypeCreate,
		Record: pm.PhotoAlbumRelationRecord{RecordName: item + string(pm.RelationSeparator) + albumID,
			RecordType:   pm.CPLContainerRelation,
			Fields:       fields,
			PluginFields: pm.PhotoMutationPluginFields{}}}},
		ZoneID: photoMutationZone(read.auth),
		Atomic: pm.PhotoAlbumRelationRequestAtomicTrue}

	var wire pm.PhotoMutationRequest
	err := wire.FromPhotoAlbumRelationRequest(input)

	if err != nil {
		return nil, read.failure(err, Configuration)
	}

	_, rejected, err := read.modifyPhoto(ctx, wire)
	if err != nil {
		return nil, err
	}

	if rejected {
		return nil, read.failure(errPhotoMutationRejected, Provider)
	}

	return &PhotoAlbumRelationResult{Added: true, Responses: read.metadata()}, nil
}

// DeletePhoto soft deletes a looked-up photo with its current provider revision.
func (sdk *SDK) DeletePhoto(ctx context.Context, request DeletePhotoRequest) (*PhotoDeletionResult, error) {
	read, err := sdk.beginPhotosMutation(ctx, request.Auth, "DeletePhoto", request.Library)
	if err != nil {
		return nil, err
	}

	master, asset, err := read.mutationPhoto(ctx, request.Album, request.PhotoID)
	if err != nil {
		return nil, err
	}

	zone := photoRecordMutationZone(read.auth, asset)

	tag := photoMutationTag(asset)
	if tag == nil {
		tag = photoMutationTag(master)
	}

	input := pm.PhotoAssetDeletionRequest{Operations: []pm.PhotoAssetDeletionOperation{{
		OperationType: pm.PhotoAssetDeletionOperationOperationTypeUpdate,
		Record: pm.PhotoAssetDeletionRecord{RecordName: asset.RecordName,
			RecordType:      pm.PhotoAssetDeletionRecordRecordTypeCPLAsset,
			Fields:          pm.PhotoDeletionFields{IsDeleted: photoMutationInteger(int64(pm.TrueFlag))},
			PluginFields:    pm.PhotoMutationPluginFields{},
			RecordChangeTag: tag,
			ZoneID:          zone}}},
		ZoneID: zone,
		Atomic: pm.PhotoAssetDeletionRequestAtomicTrue}

	var wire pm.PhotoMutationRequest
	err = wire.FromPhotoAssetDeletionRequest(input)

	if err != nil {
		return nil, read.failure(err, Configuration)
	}

	_, rejected, err := read.modifyPhoto(ctx, wire, true)
	if err != nil {
		return nil, err
	}

	if rejected {
		return nil, read.failure(errPhotoMutationRejected, Provider)
	}

	return &PhotoDeletionResult{Deleted: true, Responses: read.metadata()}, nil
}

// SetPhotoFavorite updates favorite state and confirms it from the acknowledged or refreshed asset.
func (sdk *SDK) SetPhotoFavorite(ctx context.Context,
	request SetPhotoFavoriteRequest) (*PhotoMutationResult, error) {
	read, err := sdk.beginPhotosMutation(ctx, request.Auth, "SetPhotoFavorite", request.Library)
	if err != nil {
		return nil, err
	}

	master, asset, err := read.mutationPhoto(ctx, request.Album, request.PhotoID)
	if err != nil {
		return nil, err
	}

	zone := photoRecordMutationZone(read.auth, asset)

	tag := photoMutationTag(asset)
	if tag == nil {
		tag = photoMutationTag(master)
	}

	value := int64(pm.FalseFlag)
	if request.Favorite {
		value = int64(pm.TrueFlag)
	}

	input := pm.PhotoFavoriteRequest{Operations: []pm.PhotoFavoriteOperation{{
		OperationType: pm.PhotoFavoriteOperationOperationTypeUpdate,
		Record: pm.PhotoFavoriteRecord{RecordName: asset.RecordName,
			RecordType:      pm.PhotoFavoriteRecordRecordTypeCPLAsset,
			Fields:          pm.PhotoFavoriteFields{IsFavorite: photoMutationInteger(value)},
			PluginFields:    pm.PhotoMutationPluginFields{},
			RecordChangeTag: tag,
			ZoneID:          zone}}},
		ZoneID: zone,
		Atomic: pm.PhotoFavoriteRequestAtomicTrue}

	var wire pm.PhotoMutationRequest
	err = wire.FromPhotoFavoriteRequest(input)

	if err != nil {
		return nil, read.failure(err, Configuration)
	}

	records, rejected, err := read.modifyPhoto(ctx, wire, true)
	if err != nil {
		return nil, err
	}

	return read.confirmPhotoFavorite(ctx, request, master, asset, records, rejected, value)
}
func (read *photosRead) confirmPhotoFavorite(ctx context.Context,
	request SetPhotoFavoriteRequest,
	master,
	asset cloudkit.CKRecord,
	records []cloudkit.CKRecord,
	rejected bool,
	value int64) (*PhotoMutationResult, error) {
	asset, matched := photoAcknowledgedAsset(asset, records)
	shared := request.Library != nil && request.Library.IsSharedLibrary
	if shared || !matched || rejected {
		refreshedMaster, refreshedAsset, ok, refreshErr := read.refreshMutationPhoto(ctx, request.PhotoID)
		if refreshErr == nil && ok {
			master = refreshedMaster
			asset = refreshedAsset
			rejected = false
		}
	}

	if rejected {
		return nil, read.failure(errPhotoMutationRejected, Provider)
	}

	return read.favoriteMutationResult(master, asset, value)
}
func (read *photosRead) favoriteMutationResult(master, asset cloudkit.CKRecord, value int64) (*PhotoMutationResult, error) {
	raw, err := photoFieldValue(asset, string(pm.FavoriteFieldName))
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	favorite, err := reminderInteger(raw)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	if favorite != value {
		return nil, read.failure(errPhotoMutationRejected, Provider)
	}

	photo, err := projectPhoto(master, asset)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	return &PhotoMutationResult{Photo: photo, Responses: read.metadata()}, nil
}
func photoAcknowledgedAsset(asset cloudkit.CKRecord, records []cloudkit.CKRecord) (cloudkit.CKRecord, bool) {
	for _, record := range records {
		if record.RecordName == asset.RecordName && record.RecordType == asset.RecordType {
			return record, true
		}
	}
	return asset, false
}
func (read *photosRead) refreshMutationPhoto(ctx context.Context, photoID string,
) (cloudkit.CKRecord, cloudkit.CKRecord, bool, error) {
	entry := photoAlbumEntry{album: PhotoAlbum{ID: string(cloudkit.Library), Name: string(cloudkit.Library),
		FullName: string(cloudkit.Library), RecordChangeTag: nil}, index: photoSmartIndex(cloudkit.Library),
		parent: nil, descending: false}
	return read.lookupMutationPhoto(ctx, entry, photoID)
}
