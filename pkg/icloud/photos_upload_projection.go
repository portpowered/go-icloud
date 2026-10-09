package icloud

import (
	"encoding/json"
	"maps"
	"time"

	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/photosupload"
)

func copyUploadNullable[T any](value nullable.Nullable[T]) nullable.Nullable[T] {
	result := nullable.NewNullNullable[T]()

	item, err := value.Get()
	if err == nil {
		result.Set(item)
	}

	return result
}

func uploadUnknownFields[T any](fields map[string]T) map[string]UnknownJSONValue {
	result := make(map[string]UnknownJSONValue, len(fields))

	for key, value := range fields {
		encoded, err := json.Marshal(value)
		if err == nil {
			result[key] = encoded
		}
	}

	return result
}

func projectPhotoReceipt(receipt photosupload.PhotosSingleFileUpload) PhotoUploadReceipt {
	return PhotoUploadReceipt{
		ReferenceChecksum: copyUploadNullable(receipt.ReferenceChecksum), Size: copyUploadNullable(receipt.Size),
		FileChecksum: copyUploadNullable(receipt.FileChecksum), WrappingKey: copyUploadNullable(receipt.WrappingKey),
		Receipt: copyUploadNullable(receipt.Receipt), AdditionalProperties: uploadUnknownFields(receipt.AdditionalProperties),
	}
}

func photoReceiptWire(receipt PhotoUploadReceipt) photosupload.PhotosSingleFileUpload {
	fields := make(map[string]cloudkit.CKUnknownJSON, len(receipt.AdditionalProperties))
	maps.Copy(fields, receipt.AdditionalProperties)

	return photosupload.PhotosSingleFileUpload{
		ReferenceChecksum: copyUploadNullable(receipt.ReferenceChecksum), Size: copyUploadNullable(receipt.Size),
		FileChecksum: copyUploadNullable(receipt.FileChecksum), WrappingKey: copyUploadNullable(receipt.WrappingKey),
		Receipt: copyUploadNullable(receipt.Receipt), AdditionalProperties: fields,
	}
}

func uploadInteger(value nullable.Nullable[int64]) nullable.Nullable[int] {
	result := nullable.NewNullNullable[int]()

	number, err := value.Get()

	if err == nil {
		result.Set(int(number))
	}

	return result
}

func projectPhotoRegistration(value photosupload.PhotosPutAssetResult) PhotoUploadRegistration {
	result := PhotoUploadRegistration{JobID: copyUploadNullable(value.UploadJobId),
		MasterID: copyUploadNullable(value.CplMaster), PhotoID: copyUploadNullable(value.CplAsset),
		Status: nullable.NewNullNullable[PhotoUploadRegistrationStatus](), Duplicate: false,
		AdditionalProperties: uploadUnknownFields(value.AdditionalProperties)}
	if value.Response != nil {
		result.Status.Set(PhotoUploadRegistrationStatus{Status: uploadInteger(value.Response.Status),
			Retryable:            copyUploadNullable(value.Response.IsRetryable),
			ErrorMessage:         copyUploadNullable(value.Response.ErrorMessage),
			AdditionalProperties: uploadUnknownFields(value.Response.AdditionalProperties)})

		status, _ := value.Response.Status.Get()
		result.Duplicate = status == int64(photosupload.PhotosDuplicateStatusValue)
	}

	return result
}

func photoUploadFiles(files []PhotoUploadFile) []photosupload.PhotosPutAssetFile {
	result := make([]photosupload.PhotosPutAssetFile, 0, len(files))
	for _, file := range files {
		result = append(result, photosupload.PhotosPutAssetFile{FileName: file.Filename,
			LastModDate: photoUploadUnixMilliseconds(file.ModificationTime), TimeZoneOffset: int64(file.TimeZoneOffset),
			SingleFileUploadRequest: photoReceiptWire(file.Receipt)})
	}

	return result
}

func photoUploadUnixMilliseconds(value time.Time) int64 {
	return int64((float64(value.Unix()) + float64(value.Nanosecond())/float64(time.Second)) * 1000)
}
