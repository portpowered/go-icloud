package icloud

import (
	"context"
	"encoding/json"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// DownloadPhoto retrieves a rendition without evaluating unrelated date or metadata getters.
// Unavailable rendition URLs return explicit null; available empty files return empty bytes.
func (sdk *SDK) DownloadPhoto(ctx context.Context, request DownloadPhotoRequest) (*DownloadPhotoResult, error) {
	read, err := sdk.beginPhotosRead(ctx, request.Auth, "DownloadPhoto", request.Library)
	if err != nil {
		return nil, err
	}

	albums, err := read.albums(ctx, nil)
	if err != nil {
		return nil, err
	}

	entries, err := read.projectAlbums(albums)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	entry, found := findPhotoAlbumEntry(entries, request.Album)
	if !found {
		return nil, read.failure(errPhotoAlbumMissing, NotFound)
	}

	photos, err := read.lookupPhoto(ctx, entry, request.PhotoID, projectPhotoResources)
	if err != nil {
		return nil, err
	}

	if len(photos) == 0 {
		return nil, read.failure(errPhotoAlbumMissing, NotFound)
	}

	return read.downloadPhoto(ctx, photos[0], request.Version)
}

func (read *photosRead) downloadPhoto(ctx context.Context, photo Photo,
	version *PhotoVersion,
) (*DownloadPhotoResult, error) {
	key := PhotoOriginal
	if version != nil {
		key = *version
	}

	resource, available := photo.Versions[string(key)]
	result := &DownloadPhotoResult{Content: nil, Responses: read.metadata()}
	result.Content.SetNull()

	var target *string

	if !available {
		return result, nil
	}

	err := json.Unmarshal(resource.Url, &target)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	if target == nil {
		return result, nil
	}

	response, err := read.sdk.web.DownloadPhotoContent(ctx, read.auth, *target)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, response)
	result.Content.Set(response.Body)
	result.Responses = read.metadata()

	return result, nil
}

func projectPhotoResources(master, asset cloudkit.CKRecord) (Photo, error) {
	filename, _, err := photoRecordText(master, string(cloudkit.FilenameEnc))
	if err != nil {
		return Photo{}, err
	}

	if filename == "" {
		filename = asset.RecordName
	}

	kind, err := photoKind(master, filename)
	if err != nil {
		return Photo{}, err
	}

	videoType, err := photoValue(master, string(cloudkit.ResOriginalVidComplFileType))
	if err != nil {
		return Photo{}, err
	}

	var photo Photo

	photo.ID, photo.Filename, photo.ItemType = asset.RecordName, filename, kind
	photo.IsLivePhoto = kind == Image && string(videoType) != jsonNullValue
	photo.Versions, err = photoResources(master, photo)

	return photo, err
}
