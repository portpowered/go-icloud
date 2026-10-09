package icloud

import (
	"context"
	"errors"
	"fmt"
	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/photosupload"
	"io"
	"strings"
)

var errPhotoUploadContent = errors.New("photo upload requires seekable content")

// UploadPhoto reserves, transfers and registers one file, optionally waiting for indexing.
// It never retries uncertain writes, closes caller content, or saves account credentials.
func (sdk *SDK) UploadPhoto(ctx context.Context, request UploadPhotoRequest) (*UploadPhotoResult, error) {
	const operation = "UploadPhoto"

	err := ctx.Err()
	if err != nil {
		return nil, driveContextFailure(operation, err)
	}

	size, err := photoUploadContentSize(request.Content)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	read, err := sdk.beginPhotoUpload(ctx,
		request.Auth, request.Library, operation)
	if err != nil {
		return nil, err
	}

	if request.AlbumID != nil {
		_, albumErr := read.photoUploadAlbum(ctx, *request.AlbumID)
		if albumErr != nil {
			return nil, albumErr
		}
	}

	identity, err := sdk.randomUUID()
	if err != nil {
		return nil, read.photoUploadEntropyFailure(err)
	}

	identity = strings.ToLower(identity)

	registration, err := read.uploadOnePhoto(ctx, request, identity, size, true)
	if err != nil {
		return nil, err
	}

	return read.completePhotoUpload(ctx, request, registration)
}

func (read *photosRead) completePhotoUpload(ctx context.Context, request UploadPhotoRequest,
	registration photosupload.PhotosPutAssetResult,
) (*UploadPhotoResult, error) {
	result := &UploadPhotoResult{Registration: projectPhotoRegistration(registration),
		Photo: nullable.NewNullNullable[Photo](), Indexed: false, Responses: read.metadata()}

	if request.Hydrate || request.AlbumID != nil {
		photo, hydrationErr := read.hydratePhotoUpload(ctx, request, registration)
		if hydrationErr != nil {
			return nil, hydrationErr
		}

		if len(photo) != 0 {
			result.Photo.Set(photo[0])
			result.Indexed = true
		}
	}

	result.Responses = read.metadata()

	return result, nil
}

func photoUploadContentSize(content io.ReadSeeker) (int64, error) {
	if content == nil {
		return 0, errPhotoUploadContent
	}

	position, err := content.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, fmt.Errorf("read Photos upload cursor: %w", err)
	}

	size, err := content.Seek(0, io.SeekEnd)

	_, restoreErr := content.Seek(position, io.SeekStart)
	if err != nil || restoreErr != nil {
		return 0, fmt.Errorf("size Photos content: %w", errors.Join(err, restoreErr))
	}

	return size, nil
}

func (read *photosRead) uploadOnePhoto(ctx context.Context, request UploadPhotoRequest,
	identity string, size int64,
	serviceFlow bool,
) (photosupload.PhotosPutAssetResult, error) {
	var result photosupload.PhotosPutAssetResult

	uploadAuth := read.uploadBoundary(request.Auth)

	response, reserve, err := read.sdk.web.PhotosReserveUploads(ctx, uploadAuth,
		photosupload.PhotosCreateUploadUrlRequest{ZoneName: photoUploadZone(read.auth),
			Assets: map[string]int64{identity: size}}, serviceFlow)
	if err != nil {
		return result, read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, response)
	if reserve.UploadUrls == nil || (*reserve.UploadUrls)[identity] == "" {
		return result, read.failure(errPhotoUploadReservation, InvalidResponse)
	}

	target := (*reserve.UploadUrls)[identity]

	response, receipt, err := read.sdk.web.PhotosUploadBytes(ctx, read.auth, target, request.Content)
	if err != nil {
		return result, read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, response)

	group := identity
	if request.ImportGroup != nil && *request.ImportGroup != "" {
		group = *request.ImportGroup
	}

	files := []photosupload.PhotosPutAssetFile{{FileName: request.Filename,
		LastModDate:    photoUploadUnixMilliseconds(request.ModificationTime),
		TimeZoneOffset: int64(request.TimeZoneOffset), SingleFileUploadRequest: receipt}}

	response, registrations, err := read.sdk.web.PhotosRegisterUploads(ctx, uploadAuth,
		photosupload.PhotosPutAssetRequest{ZoneName: photoUploadZone(read.auth), Files: files,
			LocalTimeZoneId: request.LocalTimeZoneID, ImportGroup: group}, serviceFlow)
	if err != nil {
		return result, read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, response)

	return read.singlePhotoRegistration(registrations)
}

func (read *photosRead) singlePhotoRegistration(
	registrations []photosupload.PhotosPutAssetResult,
) (photosupload.PhotosPutAssetResult, error) {
	var result photosupload.PhotosPutAssetResult
	if len(registrations) != 1 {
		return result, read.failure(errPhotoUploadRegistration, InvalidResponse)
	}

	result = registrations[0]
	if result.Response != nil {
		status, _ := result.Response.Status.Get()
		if status >= 400 && status != int64(photosupload.PhotosDuplicateStatusValue) {
			return result, read.failure(errPhotoUploadRegistration, Provider)
		}
	}

	return result, nil
}
