package icloud

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func photoAssetMetadata(record cloudkit.CKRecord) (json.RawMessage, error) {
	raw, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("encode photo metadata: %w", err)
	}

	object, err := photoMetadataRecord(raw)
	if err != nil {
		return nil, err
	}

	fields := map[string]json.RawMessage{}

	if record.Fields != nil {
		for name, field := range *record.Fields {
			encoded, err := json.Marshal(field)
			if err != nil {
				return nil, fmt.Errorf("encode photo field: %w", err)
			}

			fields[name], err = photoMetadataField(encoded)
			if err != nil {
				return nil, err
			}
		}
	}

	object[protocol.PhotosCKRecordFields], err = json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("encode photo fields: %w", err)
	}

	if _, present := object[protocol.PhotosCKRecordPluginFields]; !present {
		object[protocol.PhotosCKRecordPluginFields] = json.RawMessage(`{}`)
	}

	metadata, err := json.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("encode normalized photo metadata: %w", err)
	}

	return metadata, nil
}

func photoMetadataField(raw json.RawMessage) (json.RawMessage, error) {
	object, err := photoModelObject(raw)
	if err != nil {
		return nil, err
	}

	var tag string

	err = json.Unmarshal(object[protocol.PhotosCKPassthroughFieldType], &tag)
	if err != nil {
		return nil, fmt.Errorf("decode photo field tag: %w", err)
	}

	value, present := object[protocol.PhotosCKPassthroughFieldValue]
	if present {
		value, err = photoNormalizedValue(tag, value)
		if err != nil {
			return nil, err
		}

		if string(value) == jsonNullValue {
			delete(object, protocol.PhotosCKPassthroughFieldValue)
		} else {
			object[protocol.PhotosCKPassthroughFieldValue] = value
		}
	}

	err = photoMetadataEncryption(object, tag)
	if err != nil {
		return nil, err
	}

	encoded, err := json.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("encode photo metadata field: %w", err)
	}

	return encoded, nil
}

func photoNormalizedValue(tag string, value json.RawMessage) (json.RawMessage, error) {
	switch tag {
	case string(cloudkit.CKInt64FieldTypeINT64):
		return photoIntegerValue(value)
	case string(cloudkit.TIMESTAMP):
		return photoTimestampValue(value), nil
	case string(cloudkit.BYTES), string(cloudkit.ENCRYPTEDBYTES):
		return photoBase64Value(value)
	case string(cloudkit.DOUBLE):
		return photoFloatValue(value)
	case string(cloudkit.CKReferenceFieldTypeREFERENCE):
		return photoObjectValue(value, false)
	case string(cloudkit.ASSETID), string(cloudkit.ASSET):
		return photoObjectValue(value, true)
	default:
		return photoNormalizedList(tag, value)
	}
}

func photoModelObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage

	err := json.Unmarshal(raw, &object)
	if err != nil {
		return nil, fmt.Errorf("decode photo metadata object: %w", err)
	}

	for name, value := range object {
		if string(value) == jsonNullValue {
			delete(object, name)
		}
	}

	return object, nil
}

func photoFloatValue(value json.RawMessage) (json.RawMessage, error) {
	number, numeric := photoOptionalNumber(value)
	if !numeric {
		var text string

		decodeErr := json.Unmarshal(value, &text)
		if decodeErr != nil {
			return nil, fmt.Errorf("decode photo number string: %w", decodeErr)
		}

		parsed, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil {
			return nil, fmt.Errorf("decode photo number: %w", err)
		}

		number = parsed
	}

	encoded, err := json.Marshal(number)
	if err != nil {
		return nil, fmt.Errorf("encode photo number: %w", err)
	}

	if !bytes.ContainsAny(encoded, ".eE") {
		encoded = append(encoded, '.', '0')
	}

	return encoded, nil
}

func photoObjectValue(value json.RawMessage, asset bool) (json.RawMessage, error) {
	object, err := photoModelObject(value)
	if err != nil {
		return nil, err
	}

	if !asset {
		err = photoNormalizeChild(object, protocol.PhotosCKReferenceZoneID, photoPlainModel)
		if err != nil {
			return nil, err
		}
	}

	if asset {
		err = photoNormalizeChild(object, protocol.PhotosCKAssetTokenDownloadedData, photoBase64Value)
		if err != nil {
			return nil, err
		}

		if raw, present := object[protocol.PhotosCKAssetTokenSize]; present {
			integer, err := photoIntegerValue(raw)
			if err != nil {
				return nil, err
			}

			object[protocol.PhotosCKAssetTokenSize] = integer
		}
	}

	encoded, err := json.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("encode photo object: %w", err)
	}

	return encoded, nil
}

func photoNormalizedList(tag string, value json.RawMessage) (json.RawMessage, error) {
	switch tag {
	case string(cloudkit.INT64LIST):
		return photoValueList(value, string(cloudkit.CKInt64FieldTypeINT64))
	case string(cloudkit.DOUBLELIST):
		return photoValueList(value, string(cloudkit.DOUBLE))
	case string(cloudkit.CKReferenceListFieldTypeREFERENCELIST):
		return photoValueList(value, string(cloudkit.CKReferenceFieldTypeREFERENCE))
	case string(cloudkit.ASSETIDLIST):
		return photoValueList(value, string(cloudkit.ASSETID))
	default:
		return value, nil
	}
}

func photoMetadataEncryption(object map[string]json.RawMessage, tag string) error {
	if tag != string(cloudkit.CKStringFieldTypeSTRING) && tag != string(cloudkit.DOUBLE) {
		return nil
	}

	raw, present := object[protocol.PhotosCKStringFieldIsEncrypted]
	if !present {
		return nil
	}

	var text string
	if json.Unmarshal(raw, &text) != nil {
		text = string(raw)
	}

	truth, err := regexp.MatchString(protocol.RemindersCKBooleanTrueInputTextPattern, text)
	if err != nil {
		return fmt.Errorf("decode photo encryption flag: %w", err)
	}

	number, numeric := photoOptionalNumber(raw)
	if numeric {
		truth = number == 1
	}

	object[protocol.PhotosCKStringFieldIsEncrypted] = json.RawMessage(jsonFalseValue)
	if truth {
		object[protocol.PhotosCKStringFieldIsEncrypted] = json.RawMessage(jsonTrueValue)
	}

	return nil
}

func photoIntegerValue(raw json.RawMessage) (json.RawMessage, error) {
	if string(raw) == jsonTrueValue {
		return json.RawMessage("1"), nil
	}

	if string(raw) == jsonFalseValue {
		return json.RawMessage("0"), nil
	}

	text := string(raw)

	var decoded string

	if json.Unmarshal(raw, &decoded) == nil {
		text = strings.ReplaceAll(strings.TrimSpace(decoded), "_", "")
	} else if strings.ContainsAny(text, ".eE") {
		return photoFloatIntegerValue(text)
	}

	number, valid := new(big.Rat).SetString(text)
	if !valid || !number.IsInt() {
		return nil, errPhotoCount
	}

	return json.RawMessage(number.Num().String()), nil
}

func photoFloatIntegerValue(text string) (json.RawMessage, error) {
	value, err := strconv.ParseFloat(text, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return nil, errPhotoCount
	}

	if value <= math.MinInt64 || value >= float64(math.MaxInt64) || math.Trunc(value) != value {
		return nil, errPhotoCount
	}

	return json.RawMessage(strconv.FormatInt(int64(value), 10)), nil
}
