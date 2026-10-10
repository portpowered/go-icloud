package icloud

import "context"

func (read *photosRead) photoUploadAlbum(ctx context.Context, selector string) (PhotoAlbum, error) {
	const albumDiscoveryAttempts = 2
	for range albumDiscoveryAttempts {
		records, err := read.albums(ctx, nil)
		if err != nil {
			return PhotoAlbum{}, err
		}

		entries, err := projectPhotoAlbums(records)
		if err != nil {
			return PhotoAlbum{}, read.failure(err, InvalidResponse)
		}

		entry, found := findPhotoAlbumEntry(entries, selector)
		if found {
			return entry.album, nil
		}
	}

	return PhotoAlbum{}, read.failure(errPhotoAlbumMissing, NotFound)
}
