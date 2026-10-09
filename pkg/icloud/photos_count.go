package icloud

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

var errPhotoAlbumMissing = errors.New("photo album does not exist")
var errPhotoCount = errors.New("photo count is missing or invalid")

// GetPhotoAlbumCount discovers a primary album and reads its indexed photo count.
func (sdk *SDK) GetPhotoAlbumCount(
	ctx context.Context, request GetPhotoAlbumCountRequest,
) (*GetPhotoAlbumCountResult, error) {
	read, err := sdk.beginPhotosRead(ctx, request.Auth, "GetPhotoAlbumCount")
	if err != nil {
		return nil, err
	}

	records, err := read.albums(ctx, nil)
	if err != nil {
		return nil, err
	}

	albums, err := projectPhotoAlbums(records)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	index, found := findPhotoAlbumIndex(albums, request.Album)
	if !found {
		return nil, read.failure(errPhotoAlbumMissing, NotFound)
	}

	response, err := sdk.web.PhotosAlbumCount(ctx, read.auth, index)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, response.Metadata)

	count, err := photoCount(response.Data)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	return &GetPhotoAlbumCountResult{Count: count, Responses: read.metadata()}, nil
}

func photoCount(data cloudkit.PhotosCountResponse) (int64, error) {
	if data.Batch == nil || len(*data.Batch) == 0 {
		return 0, errPhotoCount
	}

	records := (*data.Batch)[0].Records
	if records == nil || len(*records) == 0 {
		return 0, errPhotoCount
	}

	raw, err := json.Marshal((*records)[0].Fields.ItemCount.Value)
	if err != nil {
		return 0, errPhotoCount
	}
	// Source integer strings permit separators after full wire validation.
	var text string
	if json.Unmarshal(raw, &text) == nil {
		raw, err = json.Marshal(strings.ReplaceAll(text, "_", ""))
		if err != nil {
			return 0, errPhotoCount
		}
	}

	count, err := reminderWireInteger(raw)
	if err != nil || count < 0 {
		return 0, errPhotoCount
	}

	return count, nil
}

func findPhotoAlbumIndex(albums []photoAlbumEntry, selector string) (string, bool) {
	for _, entry := range albums {
		if entry.album.ID == selector {
			return entry.index, true
		}
	}

	for _, entry := range albums {
		if entry.album.Name == selector || entry.album.FullName == selector {
			return entry.index, true
		}
	}

	return "", false
}

func photoSmartIndex(name cloudkit.PhotoSmartAlbumName) string {
	indexes := map[cloudkit.PhotoSmartAlbumName]cloudkit.PhotoObjectIndex{
		cloudkit.Library:         cloudkit.CPLAssetByAssetDateWithoutHiddenOrDeleted,
		cloudkit.TimeLapse:       cloudkit.CPLAssetInSmartAlbumByAssetDateTimelapse,
		cloudkit.Videos:          cloudkit.CPLAssetInSmartAlbumByAssetDateVideo,
		cloudkit.SloMo:           cloudkit.CPLAssetInSmartAlbumByAssetDateSlomo,
		cloudkit.Bursts:          cloudkit.CPLAssetBurstStackAssetByAssetDate,
		cloudkit.Favorites:       cloudkit.CPLAssetInSmartAlbumByAssetDateFavorite,
		cloudkit.Panoramas:       cloudkit.CPLAssetInSmartAlbumByAssetDatePanorama,
		cloudkit.Screenshots:     cloudkit.CPLAssetInSmartAlbumByAssetDateScreenshot,
		cloudkit.Live:            cloudkit.CPLAssetInSmartAlbumByAssetDateLive,
		cloudkit.RecentlyDeleted: cloudkit.CPLAssetDeletedByExpungedDate,
		cloudkit.Hidden:          cloudkit.CPLAssetHiddenByAssetDate,
	}

	return string(indexes[name])
}
