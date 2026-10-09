package icloud

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

var errSharedFilename = errors.New("invalid shared photo filename")

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
