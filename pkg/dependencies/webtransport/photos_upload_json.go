package webtransport

import (
	"fmt"
	"maps"
	"strings"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/photosupload"
)

func orderedPhotoRegistration(payload photosupload.PhotosPutAssetRequest) ([]byte, error) {
	files := make([]string, 0, len(payload.Files))

	for _, file := range payload.Files {
		encoded, err := orderedPhotoRegistrationFile(file)
		if err != nil {
			return nil, err
		}

		files = append(files, string(encoded))
	}

	zone, err := referenceJSON(payload.ZoneName)
	if err != nil {
		return nil, err
	}

	timezone, err := referenceJSON(payload.LocalTimeZoneId)
	if err != nil {
		return nil, err
	}

	group, err := referenceJSON(payload.ImportGroup)
	if err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf("{%q: %s, %q: [%s], %q: %s, %q: %s}",
		protocol.PhotosUploadPhotosPutAssetRequestZoneName, zone,
		protocol.PhotosUploadPhotosPutAssetRequestFiles, strings.Join(files, ", "),
		protocol.PhotosUploadPhotosPutAssetRequestLocalTimeZoneId, timezone,
		protocol.PhotosUploadPhotosPutAssetRequestImportGroup, group)), nil
}

func orderedPhotoRegistrationFile(file photosupload.PhotosPutAssetFile) ([]byte, error) {
	receipt := file.SingleFileUploadRequest
	receipt.ReferenceChecksum = maps.Clone(receipt.ReferenceChecksum)
	receipt.Size = maps.Clone(receipt.Size)
	receipt.FileChecksum = maps.Clone(receipt.FileChecksum)
	receipt.WrappingKey = maps.Clone(receipt.WrappingKey)

	receipt.Receipt = maps.Clone(receipt.Receipt)

	if !receipt.ReferenceChecksum.IsSpecified() {
		receipt.ReferenceChecksum.SetNull()
	}

	if !receipt.Size.IsSpecified() {
		receipt.Size.SetNull()
	}

	if !receipt.FileChecksum.IsSpecified() {
		receipt.FileChecksum.SetNull()
	}

	if !receipt.WrappingKey.IsSpecified() {
		receipt.WrappingKey.SetNull()
	}

	if !receipt.Receipt.IsSpecified() {
		receipt.Receipt.SetNull()
	}

	encoded, err := referenceJSONFields(receipt, []string{
		protocol.PhotosUploadPhotosSingleFileUploadReferenceChecksum, protocol.PhotosUploadPhotosSingleFileUploadSize,
		protocol.PhotosUploadPhotosSingleFileUploadFileChecksum, protocol.PhotosUploadPhotosSingleFileUploadWrappingKey,
		protocol.PhotosUploadPhotosSingleFileUploadReceipt,
	})
	if err != nil {
		return nil, err
	}

	name, err := referenceJSON(file.FileName)
	if err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf("{%q: %s, %q: %d, %q: %d, %q: %s}",
		protocol.PhotosUploadPhotosPutAssetFileFileName, name,
		protocol.PhotosUploadPhotosPutAssetFileLastModDate, file.LastModDate,
		protocol.PhotosUploadPhotosPutAssetFileTimeZoneOffset, file.TimeZoneOffset,
		protocol.PhotosUploadPhotosPutAssetFileSingleFileUploadRequest, encoded)), nil
}
