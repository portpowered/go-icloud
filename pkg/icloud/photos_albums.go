package icloud

import (
	"context"
	"encoding/json"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

type photoAlbumEntry struct {
	album      PhotoAlbum
	parent     *string
	descending bool
}

// ListPhotoAlbums initializes the primary library and discovers complete ordered albums.
func (sdk *SDK) ListPhotoAlbums(ctx context.Context, request ListPhotoAlbumsRequest) (*ListPhotoAlbumsResult, error) {
	read, err := sdk.beginPhotosRead(ctx, request.Auth, "ListPhotoAlbums")
	if err != nil {
		return nil, err
	}

	records, err := read.albums(ctx, nil)
	if err != nil {
		return nil, err
	}

	entries, err := projectPhotoAlbums(records)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	albums := make([]PhotoAlbum, 0, len(entries))
	for _, entry := range entries {
		albums = append(albums, entry.album)
	}

	return &ListPhotoAlbumsResult{Albums: albums, Responses: read.metadata()}, nil
}

func projectPhotoAlbums(records []cloudkit.CKRecord) ([]photoAlbumEntry, error) {
	entries := []photoAlbumEntry{}
	positions := map[string]int{}

	for _, name := range []cloudkit.PhotoSmartAlbumName{cloudkit.Library, cloudkit.TimeLapse, cloudkit.Videos,
		cloudkit.SloMo, cloudkit.Bursts, cloudkit.Favorites, cloudkit.Panoramas, cloudkit.Screenshots, cloudkit.Live,
		cloudkit.RecentlyDeleted, cloudkit.Hidden} {
		album := PhotoAlbum{ID: string(name), Name: string(name), FullName: string(name), RecordChangeTag: nil}
		album.RecordChangeTag.SetNull()

		positions[album.ID] = len(entries)
		entries = append(entries, photoAlbumEntry{album: album, parent: nil, descending: false})
	}

	for _, record := range records {
		entry, present, err := photoAlbumFromRecord(record)
		if err != nil {
			return nil, err
		}

		if !present {
			continue
		}

		if index, exists := positions[entry.album.ID]; exists {
			entries[index] = entry
		} else {
			positions[entry.album.ID] = len(entries)
			entries = append(entries, entry)
		}
	}

	for index := range entries {
		entries[index].album.FullName = photoAlbumFullName(entries, positions, index, map[string]bool{})
	}

	return entries, nil
}

func photoAlbumFromRecord(record cloudkit.CKRecord) (photoAlbumEntry, bool, error) {
	var absent photoAlbumEntry

	name, ok, err := photoRecordText(record, protocol.PhotosPhotoAlbumNameFieldValue)
	if err != nil {
		return absent, false, err
	}

	if !ok {
		return absent, false, nil
	}

	raw, err := reminderField(record, protocol.PhotosPhotoAlbumDeletedFieldValue)
	if err != nil {
		return absent, false, err
	}

	deleted, err := reminderTruthy(raw)
	if err != nil {
		return absent, false, err
	}

	if deleted {
		return absent, false, nil
	}

	entry, err := photoAlbumDetails(record, name)

	return entry, true, err
}

func photoAlbumDetails(record cloudkit.CKRecord, name string) (photoAlbumEntry, error) {
	entry := photoAlbumEntry{album: PhotoAlbum{ID: record.RecordName, Name: name, FullName: name,
		RecordChangeTag: record.RecordChangeTag}, parent: nil, descending: false}
	if !entry.album.RecordChangeTag.IsSpecified() {
		entry.album.RecordChangeTag.SetNull()
	}

	raw, err := reminderField(record, protocol.PhotosPhotoAlbumSortAscendingFieldValue)
	if err != nil {
		return photoAlbumEntry{}, err
	}

	if len(raw) != 0 && string(raw) != jsonNullValue {
		value, err := reminderInteger(raw)
		if err != nil {
			return photoAlbumEntry{}, err
		}

		entry.descending = value != 1
	}

	raw, err = reminderField(record, protocol.PhotosPhotoAlbumParentFieldValue)
	if err != nil {
		return photoAlbumEntry{}, err
	}

	var parent string
	if len(raw) != 0 && json.Unmarshal(raw, &parent) == nil && string(raw) != jsonNullValue {
		entry.parent = &parent
	}

	return entry, nil
}

func photoAlbumFullName(entries []photoAlbumEntry, positions map[string]int, index int, seen map[string]bool) string {
	entry := entries[index]
	if seen[entry.album.ID] || entry.parent == nil {
		return entry.album.Name
	}

	seen[entry.album.ID] = true

	parent, exists := positions[*entry.parent]

	if !exists || seen[*entry.parent] {
		return entry.album.Name
	}

	prefix := photoAlbumFullName(entries, positions, parent, seen)
	if prefix == "" {
		return entry.album.Name
	}

	return prefix + "/" + entry.album.Name
}
