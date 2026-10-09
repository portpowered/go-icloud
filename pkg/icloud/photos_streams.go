package icloud

import (
	"context"
	"strconv"
	"time"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/sharedphotos"
)

func (sdk *SDK) beginSharedPhotos(ctx context.Context, auth AuthContext, operation string,
) (*photosRead, []sharedphotos.SharedAlbum, error) {
	err := ctx.Err()
	if err != nil {
		return nil, nil, driveContextFailure(operation, err)
	}

	read, err := sdk.beginPhotosRead(ctx, auth, operation)
	if err != nil {
		return nil, nil, err
	}

	if auth.SharedPhotosServiceURL == "" {
		return nil, nil, read.failure(errPhotoAlbumMissing, Unavailable)
	}

	read.auth.Origin = auth.SharedPhotosServiceURL

	response, err := sdk.web.PhotosSharedAlbums(ctx, read.auth)
	if err != nil {
		return nil, nil, read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, response.Metadata)

	return read, response.Data.Albums, nil
}

func selectSharedAlbum(albums []sharedphotos.SharedAlbum, selector string) (sharedphotos.SharedAlbum, bool) {
	for _, album := range albums {
		if album.Albumguid == selector {
			return album, true
		}
	}

	for _, album := range albums {
		if album.Attributes.Name == selector {
			return album, true
		}
	}

	var absent sharedphotos.SharedAlbum

	return absent, false
}

// ListSharedPhotoAlbums discovers legacy shared streams independently of CloudKit shared libraries.
func (sdk *SDK) ListSharedPhotoAlbums(ctx context.Context,
	request ListSharedPhotoAlbumsRequest,
) (*ListSharedPhotoAlbumsResult, error) {
	read, albums, err := sdk.beginSharedPhotos(ctx, request.Auth, "ListSharedPhotoAlbums")
	if err != nil {
		return nil, err
	}

	result := &ListSharedPhotoAlbumsResult{Albums: []SharedPhotoAlbum{}, Responses: read.metadata()}
	for _, album := range albums {
		result.Albums = append(result.Albums, projectSharedAlbum(album))
	}

	return result, nil
}

func projectSharedAlbum(album sharedphotos.SharedAlbum) SharedPhotoAlbum {
	millis, err := strconv.ParseInt(album.Attributes.CreationDate, 10, 64)
	if err != nil {
		millis = 0
	}

	created := time.UnixMilli(millis).UTC()
	if created.Year() < 1 || created.Year() > 9999 {
		created = time.Unix(0, 0).UTC()
	}

	result := SharedPhotoAlbum{ID: album.Albumguid, Name: album.Attributes.Name, FullName: album.Attributes.Name,
		Location: album.Albumlocation, ChangeTag: album.Albumctag, OwnerID: album.Ownerdsid,
		SharingType: album.Sharingtype, Created: created, AllowContributions: album.Attributes.Allowcontributions,
		IsPublic: album.Attributes.Ispublic, IsWebUploadSupported: album.Iswebuploadsupported, PublicURL: nil}
	result.PublicURL.SetNull()

	value, err := album.Publicurl.Get()
	if err == nil {
		result.PublicURL.Set(value)
	}

	return result
}

// CountSharedPhotos reads a discovered shared stream's asset count.
func (sdk *SDK) CountSharedPhotos(ctx context.Context, request CountSharedPhotosRequest,
) (*CountSharedPhotosResult, error) {
	read, albums, err := sdk.beginSharedPhotos(ctx, request.Auth, "CountSharedPhotos")
	if err != nil {
		return nil, err
	}

	album, found := selectSharedAlbum(albums, request.Album)
	if !found {
		return nil, read.failure(errPhotoAlbumMissing, NotFound)
	}

	response, err := sdk.web.PhotosSharedCount(ctx, read.auth, album)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, response.Metadata)

	return &CountSharedPhotosResult{Count: response.Data.Albumassetcount, Responses: read.metadata()}, nil
}

// ListSharedPhotos enumerates a shared stream with Source overlap deduplication.
func (sdk *SDK) ListSharedPhotos(ctx context.Context, request ListSharedPhotosRequest,
) (*ListSharedPhotosResult, error) {
	read, photos, err := sdk.readSharedPhotos(ctx, request.Auth, request.Album, nil, "ListSharedPhotos", true)
	if err != nil {
		return nil, err
	}

	return &ListSharedPhotosResult{Photos: photos, Responses: read.metadata()}, nil
}

// GetSharedPhoto pages a shared stream by identifier; missing assets return explicit null.
func (sdk *SDK) GetSharedPhoto(ctx context.Context, request GetSharedPhotoRequest,
) (*GetSharedPhotoResult, error) {
	read, photos, err := sdk.readSharedPhotos(ctx, request.Auth, request.Album, &request.PhotoID, "GetSharedPhoto", true)
	if err != nil {
		return nil, err
	}

	result := &GetSharedPhotoResult{Photo: nil, Responses: read.metadata()}
	result.Photo.SetNull()

	if len(photos) != 0 {
		result.Photo.Set(photos[0])
	}

	return result, nil
}

// DownloadSharedPhoto retrieves one rendition from a shared stream's provider-issued URL.
func (sdk *SDK) DownloadSharedPhoto(ctx context.Context, request DownloadSharedPhotoRequest,
) (*DownloadSharedPhotoResult, error) {
	read, photos, err := sdk.readSharedPhotos(ctx, request.Auth, request.Album, &request.PhotoID,
		"DownloadSharedPhoto", false)
	if err != nil {
		return nil, err
	}

	if len(photos) == 0 {
		return nil, read.failure(errPhotoAlbumMissing, NotFound)
	}

	result, err := read.downloadPhoto(ctx, photos[0].Photo, request.Version)
	if err != nil {
		return nil, err
	}

	return &DownloadSharedPhotoResult{Content: result.Content, Responses: result.Responses}, nil
}
