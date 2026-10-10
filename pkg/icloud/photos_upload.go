package icloud

import (
	"context"
	"errors"
	"maps"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/photosupload"
)

var errPhotoUploadReservation = errors.New("photo upload reservation has no URL for the requested file")
var errPhotoUploadRegistration = errors.New("photo registration returned invalid result count or rejected the file")

func (sdk *SDK) beginPhotoUpload(ctx context.Context, auth AuthContext, library *PhotoLibrary,
	operation string,
) (*photosRead, error) {
	err := ctx.Err()
	if err != nil {
		return nil, driveContextFailure(operation, err)
	}

	return sdk.beginPhotosRead(ctx, auth, operation, library)
}

func photoUploadZone(auth webtransport.RequestContext) string {
	if auth.PhotoZone != nil {
		return auth.PhotoZone.ZoneName
	}

	return protocol.PhotosPhotoPrimaryZoneNameValue
}

func (read *photosRead) uploadBoundary(auth AuthContext) webtransport.RequestContext {
	boundary := read.auth
	boundary.Origin = auth.PhotosUploadServiceURL

	return boundary
}

// ReservePhotoUploads reserves upload destinations for caller-selected byte lengths.
func (sdk *SDK) ReservePhotoUploads(ctx context.Context,
	request ReservePhotoUploadsRequest,
) (*ReservePhotoUploadsResult, error) {
	read, err := sdk.beginPhotoUpload(ctx, request.Auth, request.Library, "ReservePhotoUploads")
	if err != nil {
		return nil, err
	}

	response, data, err := sdk.web.PhotosReserveUploads(ctx, read.uploadBoundary(request.Auth),
		photosupload.PhotosCreateUploadUrlRequest{ZoneName: photoUploadZone(read.auth),
			Assets: maps.Clone(request.Assets)}, false)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, response)

	urls := map[string]string{}
	if data.UploadUrls != nil {
		urls = maps.Clone(*data.UploadUrls)
	}

	return &ReservePhotoUploadsResult{UploadURLs: urls, Responses: read.metadata()}, nil
}

// SendPhotoUploadBytes transfers bytes to a previously reserved HTTPS destination.
func (sdk *SDK) SendPhotoUploadBytes(ctx context.Context,
	request SendPhotoUploadBytesRequest,
) (*SendPhotoUploadBytesResult, error) {
	read, err := sdk.beginPhotoUpload(ctx, request.Auth, request.Library, "SendPhotoUploadBytes")
	if err != nil {
		return nil, err
	}

	response, data, err := sdk.web.PhotosUploadBytes(ctx, read.auth, request.URL, request.Content)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, response)

	return &SendPhotoUploadBytesResult{Receipt: projectPhotoReceipt(data), Responses: read.metadata()}, nil
}

// RegisterPhotoUploads registers stored receipts and returns each provider acknowledgement.
func (sdk *SDK) RegisterPhotoUploads(ctx context.Context,
	request RegisterPhotoUploadsRequest,
) (*RegisterPhotoUploadsResult, error) {
	read, err := sdk.beginPhotoUpload(ctx, request.Auth, request.Library, "RegisterPhotoUploads")
	if err != nil {
		return nil, err
	}

	response, data, err := sdk.web.PhotosRegisterUploads(ctx, read.uploadBoundary(request.Auth),
		photosupload.PhotosPutAssetRequest{ZoneName: photoUploadZone(read.auth), Files: photoUploadFiles(request.Files),
			LocalTimeZoneId: request.LocalTimeZoneID, ImportGroup: request.ImportGroup}, false)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, response)

	registrations := make([]PhotoUploadRegistration, 0, len(data))
	for _, item := range data {
		registrations = append(registrations, projectPhotoRegistration(item))
	}

	return &RegisterPhotoUploadsResult{Registrations: registrations, Responses: read.metadata()}, nil
}

// GetPhotoUploadStatus reads ingest progress; completed progress does not imply indexing.
func (sdk *SDK) GetPhotoUploadStatus(ctx context.Context,
	request GetPhotoUploadStatusRequest,
) (*GetPhotoUploadStatusResult, error) {
	read, err := sdk.beginPhotoUpload(ctx, request.Auth, request.Library, "GetPhotoUploadStatus")
	if err != nil {
		return nil, err
	}

	response, data, err := sdk.web.PhotosUploadStatuses(ctx, read.uploadBoundary(request.Auth),
		append([]string{}, request.JobIDs...))
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, response)

	jobs := make(map[string]PhotoUploadStatus, len(data))

	for id, item := range data {
		code, _ := item.ErrorCode.Get()
		jobs[id] = PhotoUploadStatus{Progress: uploadInteger(item.Progress), ErrorCode: uploadInteger(item.ErrorCode),
			Unknown:              code == int64(photosupload.PhotosUnknownJobErrorCodeValue),
			AdditionalProperties: uploadUnknownFields(item.AdditionalProperties)}
	}

	return &GetPhotoUploadStatusResult{Jobs: jobs, Responses: read.metadata()}, nil
}
