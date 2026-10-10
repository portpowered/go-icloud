package icloud

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/sharedphotos"
)

var errSharedFilename = errors.New("invalid shared photo filename")
var errSharedDimensions = errors.New("missing shared photo dimensions")
var errSharedPlugin = errors.New("invalid shared photo plugin fields")

func sharedFieldValue(record cloudkit.CKRecord, name string) (json.RawMessage, error) {
	if record.Fields == nil {
		return nil, nil
	}

	field, found := (*record.Fields)[name]
	if !found {
		return nil, nil
	}

	wrapper, err := field.AsCKPassthroughField()
	if err != nil {
		return nil, fmt.Errorf("decode shared photo field: %w", err)
	}

	return wrapper.Value, nil
}

func sharedPhotoFilename(master cloudkit.CKRecord) (string, error) {
	value, err := sharedFieldValue(master, string(cloudkit.FilenameEnc))
	if err != nil {
		return "", err
	}

	var encoded string
	if json.Unmarshal(value, &encoded) != nil {
		return "", errSharedFilename
	}

	decoded, err := reminderRelatedBase64(encoded)
	if err != nil {
		return "", err
	}

	if !utf8.Valid(decoded) {
		return "", errSharedFilename
	}

	return string(decoded), nil
}

func sharedPhotoDate(record cloudkit.CKRecord, name string) time.Time {
	epoch := time.Unix(0, 0).UTC()

	value, err := sharedFieldValue(record, name)
	if err != nil {
		return epoch
	}

	number, numeric := photoOptionalNumber(value)
	if !numeric {
		return epoch
	}

	seconds, fraction := math.Modf(number / photoMillisPerSecond)

	instant := time.Unix(int64(seconds), int64(math.RoundToEven(fraction*photoMicrosPerSecond))*photoNanosPerMicro).UTC()
	if instant.Year() < 1 || instant.Year() > 9999 {
		return epoch
	}

	return instant
}

func sharedPhotoSize(master cloudkit.CKRecord, name string) json.RawMessage {
	value, err := sharedFieldValue(master, name)
	if err != nil {
		return json.RawMessage("0")
	}

	var text string
	if json.Unmarshal(value, &text) == nil {
		integer, parseErr := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
		if parseErr != nil {
			return json.RawMessage("0")
		}

		return json.RawMessage(strconv.FormatInt(integer, 10))
	}

	integer, err := photoIntegerValue(value)
	if err == nil {
		return integer
	}

	number, numeric := photoOptionalNumber(value)
	if !numeric {
		return json.RawMessage("0")
	}

	return json.RawMessage(strconv.FormatFloat(math.Trunc(number), 'f', 0, 64))
}

func sharedPhotoLikes(photo Photo, record cloudkit.CKRecord) (SharedPhoto, error) {
	result := SharedPhoto{Photo: photo, LikeCount: 0, Liked: false}
	if record.PluginFields == nil {
		return result, nil
	}

	for _, name := range []sharedphotos.SharedPluginField{sharedphotos.LikeCount, sharedphotos.LikedByCaller} {
		raw, found := (*record.PluginFields)[string(name)]
		if !found {
			continue
		}

		encoded, err := json.Marshal(raw)
		if err != nil {
			return SharedPhoto{}, fmt.Errorf("encode shared plugin: %w", err)
		}

		var wrapper map[string]json.RawMessage
		if json.Unmarshal(encoded, &wrapper) != nil || wrapper == nil {
			return SharedPhoto{}, errSharedPlugin
		}

		value, present := wrapper[protocol.SharedPhotosSharedJSONValueWrapperValue]
		if !present {
			continue
		}

		if name == sharedphotos.LikeCount {
			if string(value) == jsonNullValue || json.Unmarshal(value, &result.LikeCount) != nil {
				return SharedPhoto{}, errSharedPlugin
			}
		} else {
			result.Liked = sharedJSONTruthy(value)
		}
	}

	return result, nil
}

func sharedJSONTruthy(raw json.RawMessage) bool {
	var value any
	if json.Unmarshal(raw, &value) != nil || value == nil {
		return false
	}

	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed != 0
	case string:
		return typed != ""
	case []any:
		return len(typed) != 0
	case map[string]any:
		return len(typed) != 0
	default:
		return false
	}
}
