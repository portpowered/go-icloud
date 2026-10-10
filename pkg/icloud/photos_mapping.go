package icloud

import (
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

type photoFileSpec struct {
	kind      PhotoItemType
	extension cloudkit.PhotoFileExtension
}

func photoFileTypes() map[cloudkit.PhotoFileType]photoFileSpec {
	return map[cloudkit.PhotoFileType]photoFileSpec{
		cloudkit.PhotoFileTypePublicHeic:              {kind: Image, extension: cloudkit.DotHEIC},
		cloudkit.PhotoFileTypePublicHeif:              {kind: Image, extension: cloudkit.DotHEIF},
		cloudkit.PhotoFileTypePublicJpeg:              {kind: Image, extension: cloudkit.DotJPG},
		cloudkit.PhotoFileTypePublicPng:               {kind: Image, extension: cloudkit.DotPNG},
		cloudkit.PhotoFileTypeComAppleQuicktimeMovie:  {kind: Movie, extension: cloudkit.DotMOV},
		cloudkit.PhotoFileTypePublicMpeg4:             {kind: Movie, extension: cloudkit.DotMP4},
		cloudkit.PhotoFileTypeComAppleM4vVideo:        {kind: Movie, extension: cloudkit.DotM4V},
		cloudkit.PhotoFileTypeComAdobeRawImage:        {kind: Image, extension: cloudkit.DotDNG},
		cloudkit.PhotoFileTypeComCanonCr2RawImage:     {kind: Image, extension: cloudkit.DotCR2},
		cloudkit.PhotoFileTypeComCanonCr3RawImage:     {kind: Image, extension: cloudkit.DotCR3},
		cloudkit.PhotoFileTypeComCanonCrwRawImage:     {kind: Image, extension: cloudkit.DotCRW},
		cloudkit.PhotoFileTypeComFujiRawImage:         {kind: Image, extension: cloudkit.DotRAF},
		cloudkit.PhotoFileTypeComNikonNrwRawImage:     {kind: Image, extension: cloudkit.DotNRF},
		cloudkit.PhotoFileTypeComNikonRawImage:        {kind: Image, extension: cloudkit.DotNEF},
		cloudkit.PhotoFileTypeComOlympusOrRawImage:    {kind: Image, extension: cloudkit.DotORF},
		cloudkit.PhotoFileTypeComOlympusRawImage:      {kind: Image, extension: cloudkit.DotORF},
		cloudkit.PhotoFileTypeComPanasonicRw2RawImage: {kind: Image, extension: cloudkit.DotRW2},
		cloudkit.PhotoFileTypeComPentaxRawImage:       {kind: Image, extension: cloudkit.DotPEF},
		cloudkit.PhotoFileTypeComSonyArwRawImage:      {kind: Image, extension: cloudkit.DotARW},
	}
}

func photoImageVersions() map[cloudkit.PhotoVersionName]cloudkit.PhotoResourcePrefix {
	return map[cloudkit.PhotoVersionName]cloudkit.PhotoResourcePrefix{
		cloudkit.Original:      cloudkit.ResOriginal,
		cloudkit.Alternative:   cloudkit.ResOriginalAlt,
		cloudkit.Medium:        cloudkit.ResJPEGMed,
		cloudkit.Thumb:         cloudkit.ResJPEGThumb,
		cloudkit.OriginalVideo: cloudkit.ResOriginalVidCompl,
		cloudkit.MediumVideo:   cloudkit.ResVidMed,
		cloudkit.ThumbVideo:    cloudkit.ResVidSmall,
		cloudkit.Sidecar:       cloudkit.ResSidecar,
	}
}

func photoMovieVersions() map[cloudkit.PhotoVersionName]cloudkit.PhotoResourcePrefix {
	return map[cloudkit.PhotoVersionName]cloudkit.PhotoResourcePrefix{
		cloudkit.Original:    cloudkit.ResOriginal,
		cloudkit.Medium:      cloudkit.ResVidMed,
		cloudkit.Thumb:       cloudkit.ResVidSmall,
		cloudkit.ThumbImage:  cloudkit.ResJPEGThumb,
		cloudkit.MediumImage: cloudkit.ResJPEGMed,
	}
}

type photoAlbumQuerySpec struct {
	index     cloudkit.PhotoListIndex
	direction cloudkit.PhotoDirection
	filters   []webtransport.PhotosAssetSelector
}

func photoSmartQueries() map[cloudkit.PhotoSmartAlbumName]photoAlbumQuerySpec {
	return map[cloudkit.PhotoSmartAlbumName]photoAlbumQuerySpec{
		cloudkit.Library: {index: cloudkit.CPLAssetAndMasterByAssetDateWithoutHiddenOrDeleted,
			direction: cloudkit.DESCENDING,
			filters:   nil},
		cloudkit.TimeLapse: {index: cloudkit.CPLAssetAndMasterInSmartAlbumByAssetDate,
			direction: cloudkit.ASCENDING,
			filters: []webtransport.PhotosAssetSelector{{
				Field: cloudkit.PhotoAssetQueryFieldSmartAlbum,
				Value: string(cloudkit.TIMELAPSE)}}},
		cloudkit.Videos: {index: cloudkit.CPLAssetAndMasterInSmartAlbumByAssetDate,
			direction: cloudkit.ASCENDING,
			filters: []webtransport.PhotosAssetSelector{{
				Field: cloudkit.PhotoAssetQueryFieldSmartAlbum,
				Value: string(cloudkit.VIDEO)}}},
		cloudkit.SloMo: {index: cloudkit.CPLAssetAndMasterInSmartAlbumByAssetDate,
			direction: cloudkit.ASCENDING,
			filters: []webtransport.PhotosAssetSelector{{
				Field: cloudkit.PhotoAssetQueryFieldSmartAlbum,
				Value: string(cloudkit.SLOMO)}}},
		cloudkit.Bursts: {index: cloudkit.CPLBurstStackAssetAndMasterByAssetDate,
			direction: cloudkit.ASCENDING,
			filters:   nil},
		cloudkit.Favorites: {index: cloudkit.CPLAssetAndMasterInSmartAlbumByAssetDate,
			direction: cloudkit.ASCENDING,
			filters: []webtransport.PhotosAssetSelector{{
				Field: cloudkit.PhotoAssetQueryFieldSmartAlbum,
				Value: string(cloudkit.FAVORITE)}}},
		cloudkit.Panoramas: {index: cloudkit.CPLAssetAndMasterInSmartAlbumByAssetDate,
			direction: cloudkit.ASCENDING,
			filters: []webtransport.PhotosAssetSelector{{
				Field: cloudkit.PhotoAssetQueryFieldSmartAlbum,
				Value: string(cloudkit.PANORAMA)}}},
		cloudkit.Screenshots: {index: cloudkit.CPLAssetAndMasterInSmartAlbumByAssetDate,
			direction: cloudkit.ASCENDING,
			filters: []webtransport.PhotosAssetSelector{{
				Field: cloudkit.PhotoAssetQueryFieldSmartAlbum,
				Value: string(cloudkit.SCREENSHOT)}}},
		cloudkit.Live: {index: cloudkit.CPLAssetAndMasterInSmartAlbumByAssetDate,
			direction: cloudkit.ASCENDING,
			filters: []webtransport.PhotosAssetSelector{{
				Field: cloudkit.PhotoAssetQueryFieldSmartAlbum,
				Value: string(cloudkit.LIVE)}}},
		cloudkit.RecentlyDeleted: {index: cloudkit.CPLAssetAndMasterDeletedByExpungedDate,
			direction: cloudkit.ASCENDING,
			filters:   nil},
		cloudkit.Hidden: {index: cloudkit.CPLAssetAndMasterHiddenByAssetDate,
			direction: cloudkit.ASCENDING,
			filters:   nil},
	}
}
