package icloud

import (
	"encoding/json"
	"strings"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const photoTokenSizeKey = protocol.PhotosCKAssetTokenSize

func photoResources(master cloudkit.CKRecord, photo Photo) (map[string]PhotoResource, error) {
	mapping := photoImageVersions()
	if photo.ItemType == Movie {
		mapping = photoMovieVersions()
	}

	resources := map[string]PhotoResource{}

	for key, prefix := range mapping {
		resource, present, err := photoResource(master, photo, prefix)
		if err != nil {
			return nil, err
		}

		if present {
			resources[string(key)] = resource
		}
	}

	return resources, nil
}

func photoResource(master cloudkit.CKRecord, photo Photo,
	prefix cloudkit.PhotoResourcePrefix,
) (PhotoResource, bool, error) {
	values := []json.RawMessage{}

	for _, suffix := range []cloudkit.PhotoResourceSuffix{cloudkit.PhotoResourceSuffixRes,
		cloudkit.PhotoResourceSuffixFileType, cloudkit.PhotoResourceSuffixFingerprint,
		cloudkit.PhotoResourceSuffixWidth, cloudkit.PhotoResourceSuffixHeight} {
		value, err := photoValue(master, string(prefix)+string(suffix))
		if err != nil {
			return PhotoResource{}, false, err
		}

		values = append(values, value)
	}

	if string(values[0]) == jsonNullValue {
		var absent PhotoResource

		return absent, false, nil
	}

	filename := photoResourceFilename(photo, values[1])

	return PhotoResource{Filename: filename, Url: photoTokenProperty(values[0], protocol.PhotosCKAssetTokenDownloadURL),
		Size: photoTokenProperty(values[0], photoTokenSizeKey), Type: values[1], Checksum: values[2],
		Width: values[3], Height: values[4]}, true, nil
}

func photoTokenProperty(token json.RawMessage, name string) json.RawMessage {
	var object map[string]json.RawMessage
	if json.Unmarshal(token, &object) == nil {
		if value, exists := object[name]; exists {
			return value
		}
	}

	return json.RawMessage(jsonNullValue)
}

func photoResourceFilename(photo Photo, rawType json.RawMessage) string {
	var fileType cloudkit.PhotoFileType
	if json.Unmarshal(rawType, &fileType) != nil {
		return photo.Filename
	}

	spec, known := photoFileTypes()[fileType]
	if known && (spec.extension != "" || (photo.IsLivePhoto && spec.kind == Movie)) {
		extension := spec.extension
		if extension == "" {
			extension = cloudkit.DotMOV
		}

		return photoFilenameStem(photo.Filename) + string(extension)
	}

	return photo.Filename
}

func photoFilenameStem(filename string) string {
	separator := strings.LastIndexAny(filename, "/\\")
	dot := strings.LastIndex(filename, ".")

	first := separator + 1
	for first < len(filename) && filename[first] == '.' {
		first++
	}

	if dot >= first && dot > separator {
		return filename[:dot]
	}

	return filename
}
