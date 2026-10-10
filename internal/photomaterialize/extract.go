package photomaterialize

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"howett.net/plist"
)

const (
	maximumAdjustmentBytes = 16 << 20
	screenshotSubtype      = 3
	favoriteRating         = 5
	maximumASCII           = 127
)

// ExtractMetadata decodes optional metadata, ignoring unreadable fields as Source does.
func ExtractMetadata(record cloudkit.CKRecord) *Metadata {
	data, err := json.Marshal(record)
	if err != nil {
		return nil
	}

	return ExtractMetadataJSON(data)
}

// ExtractMetadataJSON accepts raw or normalized CloudKit asset metadata.
func ExtractMetadataJSON(data json.RawMessage) *Metadata {
	if string(data) == "null" {
		return nil
	}

	record := new(AssetRecord)
	if json.Unmarshal(data, record) != nil {
		return nil
	}

	fields := record.Fields
	if fields == nil {
		fields = new(AssetFields)
	}

	metadata := new(Metadata)
	metadata.Toolkit = string(SourceToolkit)
	metadata.Title = textField(fields.CaptionEnc)
	metadata.Description = textField(fields.ExtendedDescEnc)
	metadata.Orientation = orientation(fields.AdjustmentSimpleDataEnc)
	metadata.Keywords = keywords(fields.KeywordsEnc)
	metadata.CreateDate = createDate(fields.AssetDate, fields.TimeZoneOffset)
	metadata.Rating = rating(fields)
	location := location(fields.LocationEnc)
	metadata.GpsAltitude = location.Alt
	metadata.GpsLatitude = location.Lat
	metadata.GpsLongitude = location.Lon
	metadata.GpsSpeed = location.Speed

	metadata.GpsTimestamp = location.Timestamp

	if number(fields.AssetSubtypeV2) == screenshotSubtype {
		metadata.Make = new(string)
		*metadata.Make = "Screenshot"
		metadata.DigitalSourceType = new(string)
		*metadata.DigitalSourceType = "screenCapture"
	}

	return metadata
}

func fieldText(field *Field) (string, bool) {
	if field == nil || field.Value == nil || string(*field.Value) == "null" {
		return "", false
	}

	var value string

	ok := json.Unmarshal(*field.Value, &value) == nil

	return value, ok
}

func fieldBytes(field *Field) []byte {
	value, ok := fieldText(field)
	if !ok {
		return nil
	}

	if strings.IndexFunc(value, func(character rune) bool { return character > maximumASCII }) >= 0 {
		return []byte(value)
	}

	// Python b64decode's default discards non-alphabet characters.
	filtered := strings.Map(func(character rune) rune {
		if strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=", character) {
			return character
		}

		return -1
	}, value)

	decoded, err := base64.StdEncoding.DecodeString(filtered)
	if err != nil {
		return []byte(value)
	}

	return decoded
}

func textField(field *Field) *string {
	value, ok := fieldText(field)
	if !ok {
		return nil
	}

	if strings.IndexFunc(value, func(character rune) bool { return character > maximumASCII }) >= 0 {
		return &value
	}

	decoded := fieldBytes(field)
	if utf8.Valid(decoded) {
		value = string(decoded)
	}

	return &value
}

func orientation(field *Field) *int {
	raw := fieldBytes(field)
	if len(raw) == 0 || bytes.HasPrefix(raw, []byte("crdt")) || bytes.HasPrefix(raw, []byte("bplist00")) {
		return nil
	}

	reader := flate.NewReader(bytes.NewReader(raw))
	decoded, err := io.ReadAll(io.LimitReader(reader, maximumAdjustmentBytes+1))

	closeErr := reader.Close()

	if err != nil || closeErr != nil || len(decoded) > maximumAdjustmentBytes {
		return nil
	}

	adjustment := new(Adjustment)
	if json.Unmarshal(decoded, adjustment) != nil || adjustment.Metadata == nil {
		return nil
	}

	return adjustment.Metadata.Orientation
}

func keywords(field *Field) *[]string {
	var values KeywordValues

	_, err := plist.Unmarshal(fieldBytes(field), &values)
	if err != nil || values == nil {
		return nil
	}

	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, keywordString(value))
	}

	return &result
}

func keywordString(value any) string {
	switch item := value.(type) {
	case bool:
		if item {
			return "True"
		}

		return "False"
	case float64:
		return pythonFloat(item)
	case time.Time:
		return item.Format("2006-01-02 15:04:05")
	case string, int64, uint64:
		return fmt.Sprint(item)
	default:
		return fmt.Sprint(value)
	}
}

func location(field *Field) Location {
	result := new(Location)

	var values map[string]any

	_, err := plist.Unmarshal(fieldBytes(field), &values)
	if err != nil {
		return *result
	}

	result.Alt = plistNumber(values[string(AltitudeKey)])
	result.Lat = plistNumber(values[string(LatitudeKey)])
	result.Lon = plistNumber(values[string(LongitudeKey)])

	result.Speed = plistNumber(values[string(SpeedKey)])

	if timestamp, ok := values[string(TimestampKey)].(time.Time); ok {
		result.Timestamp = &timestamp
	}

	return *result
}

func plistNumber(value any) *float64 {
	var result float64

	switch number := value.(type) {
	case float64:
		result = number
	case uint64:
		result = float64(number)
	case int64:
		result = float64(number)
	case bool:
		if number {
			result = 1
		}
	default:
		return nil
	}

	return &result
}

func number(field *Field) float64 {
	if field == nil || field.Value == nil {
		return 0
	}

	var result float64
	if json.Unmarshal(*field.Value, &result) != nil {
		if string(*field.Value) == "true" {
			return 1
		}

		return 0
	}

	return result
}

func createDate(date, offset *Field) *time.Time {
	if date == nil || date.Value == nil {
		return nil
	}

	var timestamp float64
	if json.Unmarshal(*date.Value, &timestamp) != nil {
		var text time.Time
		if json.Unmarshal(*date.Value, &text) != nil {
			return nil
		}

		return &text
	}

	zone := time.FixedZone("", int(number(offset)))
	value := time.Unix(0, int64(timestamp*float64(time.Millisecond))).In(zone)

	return &value
}

func rating(fields *AssetFields) *int {
	value := 0

	switch {
	case number(fields.IsHidden) == 1 || number(fields.IsDeleted) == 1:
		value = -1
	case number(fields.IsFavorite) == 1:
		value = favoriteRating
	default:
		return nil
	}

	return &value
}
