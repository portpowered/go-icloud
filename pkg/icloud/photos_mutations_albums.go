package icloud

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	pm "github.com/portpowered/go-icloud/pkg/dependencymodels/photosmutations"
	"io"
	"strings"
)

// CreatePhotoAlbum creates an album or folder using injected time and entropy.
func (sdk *SDK) CreatePhotoAlbum(ctx context.Context,
	request CreatePhotoAlbumRequest) (*PhotoAlbumMutationResult, error) {
	read, err := sdk.beginPhotosMutation(ctx, request.Auth, "CreatePhotoAlbum", request.Library)
	if err != nil {
		return nil, err
	}

	name, err := sdk.randomPhotoAlbumName()
	if err != nil {
		return nil, read.failure(err, Configuration)
	}

	kind := int64(pm.AlbumKind)
	if request.Folder {
		kind = int64(pm.FolderKind)
	}

	fields := pm.PhotoAlbumCreationFields{AlbumNameEnc: photoAlbumName(request.Name),
		AlbumType:     photoMutationInteger(kind),
		IsDeleted:     photoMutationInteger(int64(pm.FalseFlag)),
		IsExpunged:    photoMutationInteger(int64(pm.FalseFlag)),
		Position:      photoMutationInteger(reminderTimestamp(sdk.clock()).Value.GetOrEmpty()),
		SortType:      photoMutationInteger(int64(pm.TrueFlag)),
		SortAscending: photoMutationInteger(int64(pm.TrueFlag))}
	input := pm.PhotoAlbumCreationRequest{Operations: []pm.PhotoAlbumCreationOperation{{
		OperationType: pm.PhotoAlbumCreationOperationOperationTypeCreate,
		Record: pm.PhotoAlbumCreationRecord{RecordName: name,
			RecordType:   pm.PhotoAlbumCreationRecordRecordTypeCPLAlbum,
			Fields:       fields,
			PluginFields: pm.PhotoMutationPluginFields{}}}},
		ZoneID: photoMutationZone(read.auth),
		Atomic: pm.PhotoAlbumCreationRequestAtomicTrue}

	var wire pm.PhotoMutationRequest
	err = wire.FromPhotoAlbumCreationRequest(input)

	if err != nil {
		return nil, read.failure(err, Configuration)
	}

	records, rejected, err := read.modifyPhoto(ctx, wire)
	if err != nil {
		return nil, err
	}

	if rejected {
		return nil, read.failure(errPhotoMutationRejected, Provider)
	}

	for _, record := range records {
		entry, present, err := photoAlbumFromRecord(record)
		if err != nil {
			return nil, read.failure(err, InvalidResponse)
		}

		if present {
			return read.albumMutationResult(&entry.album), nil
		}
	}

	return read.albumMutationResult(nil), nil
}
func photoAlbumName(value string) pm.PhotoMutationEncryptedText {
	return pm.PhotoMutationEncryptedText{Type: pm.ENCRYPTEDBYTES,
		Value: base64.StdEncoding.EncodeToString([]byte(value))}
}

// RenamePhotoAlbum looks up the current revision and returns the renamed album.
func (sdk *SDK) RenamePhotoAlbum(ctx context.Context,
	request RenamePhotoAlbumRequest) (*PhotoAlbumMutationResult, error) {
	read, err := sdk.beginPhotosMutation(ctx, request.Auth, "RenamePhotoAlbum", request.Library)
	if err != nil {
		return nil, err
	}

	entry, err := read.mutationAlbum(ctx, request.AlbumID)
	if err != nil {
		return nil, err
	}

	tag := photoAlbumMutationTag(entry.album)
	input := pm.PhotoAlbumRenameRequest{Operations: []pm.PhotoAlbumRenameOperation{{
		OperationType: pm.PhotoAlbumRenameOperationOperationTypeUpdate,
		Record: pm.PhotoAlbumRenameRecord{RecordName: request.AlbumID,
			RecordType:      pm.PhotoAlbumRenameRecordRecordTypeCPLAlbum,
			Fields:          pm.PhotoAlbumRenameFields{AlbumNameEnc: photoAlbumName(request.Name)},
			PluginFields:    pm.PhotoMutationPluginFields{},
			RecordChangeTag: tag}}},
		ZoneID: photoMutationZone(read.auth),
		Atomic: pm.PhotoAlbumRenameRequestAtomicTrue}

	var wire pm.PhotoMutationRequest
	err = wire.FromPhotoAlbumRenameRequest(input)

	if err != nil {
		return nil, read.failure(err, Configuration)
	}

	records, rejected, err := read.modifyPhoto(ctx, wire)
	if err != nil {
		return nil, err
	}

	if rejected {
		return nil, read.failure(errPhotoMutationRejected, Provider)
	}

	for _, record := range records {
		if record.RecordName == request.AlbumID && record.RecordChangeTag.GetOrEmpty() != "" {
			entry.album.RecordChangeTag = record.RecordChangeTag
		}
	}

	entry.album.FullName = strings.TrimSuffix(entry.album.FullName, entry.album.Name) + request.Name
	entry.album.Name = request.Name

	return read.albumMutationResult(&entry.album), nil
}

// DeletePhotoAlbum soft deletes a custom album at its discovered current revision.
func (sdk *SDK) DeletePhotoAlbum(ctx context.Context,
	request DeletePhotoAlbumRequest) (*PhotoDeletionResult, error) {
	read, err := sdk.beginPhotosMutation(ctx, request.Auth, "DeletePhotoAlbum", request.Library)
	if err != nil {
		return nil, err
	}

	entry, err := read.mutationAlbum(ctx, request.AlbumID)
	if err != nil {
		return nil, err
	}

	tag := photoAlbumMutationTag(entry.album)
	input := pm.PhotoAlbumDeletionRequest{Operations: []pm.PhotoAlbumDeletionOperation{{
		OperationType: pm.PhotoAlbumDeletionOperationOperationTypeUpdate,
		Record: pm.PhotoAlbumDeletionRecord{RecordName: request.AlbumID,
			RecordType:      pm.PhotoAlbumDeletionRecordRecordTypeCPLAlbum,
			Fields:          pm.PhotoDeletionFields{IsDeleted: photoMutationInteger(int64(pm.TrueFlag))},
			PluginFields:    pm.PhotoMutationPluginFields{},
			RecordChangeTag: tag}}},
		ZoneID: photoMutationZone(read.auth),
		Atomic: pm.PhotoAlbumDeletionRequestAtomicTrue}

	var wire pm.PhotoMutationRequest
	err = wire.FromPhotoAlbumDeletionRequest(input)

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

	return &PhotoDeletionResult{Deleted: true, Responses: read.metadata()}, nil
}

func (sdk *SDK) randomPhotoAlbumName() (string, error) {
	var raw [uuidByteCount]byte
	_, err := io.ReadFull(sdk.random, raw[:])
	if err != nil {
		return "", fmt.Errorf("read album identity entropy: %w", err)
	}
	return strings.ToUpper(hex.EncodeToString(raw[:])), nil
}
func (read *photosRead) albumMutationResult(album *PhotoAlbum) *PhotoAlbumMutationResult {
	result := &PhotoAlbumMutationResult{Album: nil, Responses: read.metadata()}
	if album == nil {
		result.Album.SetNull()
	} else {
		result.Album.Set(*album)
	}
	return result
}
