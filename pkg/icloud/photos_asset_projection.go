package icloud

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/portpowered/go-icloud/internal/remindersdate"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const photoMillisPerSecond = 1000
const photoMicrosPerSecond = 1000000
const photoNanosPerMicro = 1000

func projectPhoto(master, asset cloudkit.CKRecord) (Photo, error) {
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

	created, err := photoDate(asset, cloudkit.AssetDate)
	if err != nil {
		return Photo{}, err
	}

	added, err := photoDate(asset, cloudkit.AddedDate)
	if err != nil {
		return Photo{}, err
	}

	values, err := photoMasterValues(master, []cloudkit.PhotoMasterField{cloudkit.ResOriginalRes,
		cloudkit.ResOriginalWidth, cloudkit.ResOriginalHeight, cloudkit.ResOriginalVidComplFileType})
	if err != nil {
		return Photo{}, err
	}

	metadata, err := photoAssetMetadata(asset)
	if err != nil {
		return Photo{}, err
	}

	photo := Photo{ID: asset.RecordName, MasterID: master.RecordName, Filename: filename,
		Size: photoTokenProperty(values[0], photoTokenSizeKey), Dimensions: []UnknownJSONValue{values[1], values[2]},
		ItemType: kind, IsLivePhoto: kind == Image && string(values[3]) != jsonNullValue,
		Created: created, Added: added, Versions: map[string]PhotoResource{}, AssetMetadata: metadata}

	versions, err := photoResources(master, photo)
	if err != nil {
		return Photo{}, err
	}

	photo.Versions = versions

	return photo, nil
}

func photoMasterValues(record cloudkit.CKRecord, names []cloudkit.PhotoMasterField) ([]json.RawMessage, error) {
	result := make([]json.RawMessage, 0, len(names))

	for _, name := range names {
		value, err := photoValue(record, string(name))
		if err != nil {
			return nil, err
		}

		result = append(result, value)
	}

	return result, nil
}

func photoKind(master cloudkit.CKRecord, filename string) (PhotoItemType, error) {
	values, err := photoMasterValues(master, []cloudkit.PhotoMasterField{cloudkit.ItemType, cloudkit.ResOriginalFileType})
	if err != nil {
		return Image, err
	}

	for _, value := range values {
		var text cloudkit.PhotoFileType
		if json.Unmarshal(value, &text) == nil {
			if spec, known := photoFileTypes()[text]; known {
				return spec.kind, nil
			}
		}
	}

	var originalType string
	if json.Unmarshal(values[1], &originalType) == nil && strings.Contains(strings.ToLower(originalType), "raw") {
		return Image, nil
	}

	name := strings.ToLower(filename)
	for _, extension := range []string{".heic", ".png", ".jpg", ".jpeg", ".arw", ".cr2", ".cr3", ".crw",
		".dng", ".nef", ".nrf", ".nrw", ".orf", ".pef", ".raf", ".rw2"} {
		if strings.HasSuffix(name, extension) {
			return Image, nil
		}
	}

	return Movie, nil
}

func photoDate(record cloudkit.CKRecord, name cloudkit.PhotoMasterField) (time.Time, error) {
	value, err := photoFieldValue(record, string(name))
	if err != nil {
		return time.Time{}, err
	}

	tag, err := photoFieldType(record, string(name))
	if err != nil {
		return time.Time{}, err
	}

	epoch := time.Unix(0, 0).UTC()

	if tag == string(cloudkit.TIMESTAMP) {
		if instant := reminderDate(value); instant != nil {
			return *instant, nil
		}

		return epoch, nil
	}

	value, err = photoValue(record, string(name))
	if err != nil {
		return time.Time{}, err
	}

	number, numeric := photoOptionalNumber(value)
	if !numeric {
		return epoch, nil
	}

	seconds, fraction := math.Modf(number / photoMillisPerSecond)

	instant := time.Unix(int64(seconds), int64(math.RoundToEven(fraction*photoMicrosPerSecond))*photoNanosPerMicro).UTC()
	if instant.Year() < 1 || instant.Year() > 9999 || remindersdate.FromMillis(instant.UnixMilli()) == nil {
		return time.Time{}, errPhotoCount
	}

	return instant, nil
}

func photoOptionalNumber(value json.RawMessage) (float64, bool) {
	if string(value) == jsonTrueValue {
		return 1, true
	}

	if string(value) == jsonFalseValue {
		return 0, true
	}

	var number float64

	err := json.Unmarshal(value, &number)

	return number, err == nil && string(value) != jsonNullValue
}

func photoValue(record cloudkit.CKRecord, name string) (json.RawMessage, error) {
	value, err := photoFieldValue(record, name)
	if err != nil {
		return nil, err
	}

	if len(value) == 0 {
		return json.RawMessage(jsonNullValue), nil
	}

	tag, err := photoFieldType(record, name)
	if err != nil {
		return nil, err
	}

	return photoNormalizedValue(tag, value)
}

// Keep the reference timestamp conversion available for normalized metadata.
func photoTimestampValue(value json.RawMessage) json.RawMessage {
	instant := reminderDate(value)
	if instant == nil {
		return json.RawMessage(jsonNullValue)
	}

	seconds := float64(instant.Unix()) + float64(instant.Nanosecond())/float64(time.Second)

	return json.RawMessage(strconv.FormatInt(int64(seconds*photoMillisPerSecond), 10))
}
