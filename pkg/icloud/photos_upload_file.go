package icloud

import (
	"context"
	"strings"
)

// UploadPhotoFile reserves, transfers and registers one file without waiting for indexing.
// The caller owns the seekable reader and the returned account metadata.
func (sdk *SDK) UploadPhotoFile(ctx context.Context,
	request UploadPhotoFileRequest,
) (*UploadPhotoFileResult, error) {
	const operation = "UploadPhotoFile"

	err := ctx.Err()
	if err != nil {
		return nil, driveContextFailure(operation, err)
	}

	size, err := photoUploadContentSize(request.Content)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	read, err := sdk.beginPhotoUpload(ctx, request.Auth, request.Library, operation)
	if err != nil {
		return nil, err
	}

	identity, err := sdk.randomUUID()
	if err != nil {
		return nil, read.photoUploadEntropyFailure(err)
	}

	input := new(UploadPhotoRequest)
	input.Auth, input.Library = request.Auth, request.Library
	input.Content, input.Filename = request.Content, request.Filename
	input.ModificationTime, input.ImportGroup = request.ModificationTime, request.ImportGroup
	input.LocalTimeZoneID, input.TimeZoneOffset = request.LocalTimeZoneID, request.TimeZoneOffset

	registration, err := read.uploadOnePhoto(ctx, *input, strings.ToLower(identity), size, false)
	if err != nil {
		return nil, err
	}

	return &UploadPhotoFileResult{Registration: projectPhotoRegistration(registration), Responses: read.metadata()}, nil
}

func (read *photosRead) photoUploadEntropyFailure(err error) *ClientError {
	failure := newClientError(read.operation, Configuration, 0, nil, nil, err)
	failure.prior = read.metadata()

	return failure
}
