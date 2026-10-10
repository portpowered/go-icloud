package photosync

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const dateFormatRecursionLimit = 2

type dateFormatState struct {
	instant       time.Time
	automatic     int
	manual        bool
	usedAutomatic bool
}

type dateReplacement struct {
	name       string
	conversion byte
	spec       string
}

func (state *dateFormatState) render(format string, depth int) (string, error) {
	if depth > dateFormatRecursionLimit {
		return "", errFolderFormat
	}

	var output strings.Builder

	for index := 0; index < len(format); {
		next, text, err := state.segment(format, index, depth)
		if err != nil {
			return "", err
		}

		output.WriteString(text)

		index = next
	}

	return output.String(), nil
}

func (state *dateFormatState) segment(format string, index, depth int) (int, string, error) {
	character := format[index]
	if character != '{' && character != '}' {
		return index + 1, string(character), nil
	}

	if index+1 < len(format) && format[index+1] == character {
		return index + 2, string(character), nil
	}

	if character == '}' {
		return 0, "", errFolderFormat
	}

	end, err := replacementEnd(format, index+1)
	if err != nil {
		return 0, "", err
	}

	text, err := state.replacement(format[index+1:end], depth)

	return end + 1, text, err
}

func replacementEnd(format string, index int) (int, error) {
	nesting := 0

	for cursor := index; cursor < len(format); cursor++ {
		switch format[cursor] {
		case '{':
			nesting++
		case '}':
			if nesting == 0 {
				return cursor, nil
			}

			nesting--
		}
	}

	return 0, errFolderFormat
}

func parseReplacement(field string) (dateReplacement, error) {
	result := new(dateReplacement)
	name, spec, hasSpec := strings.Cut(field, ":")
	if hasSpec {
		result.spec = spec
	}

	name, conversion, hasConversion := strings.Cut(name, "!")
	if hasConversion {
		if len(conversion) != 1 || !strings.ContainsRune("sra", rune(conversion[0])) {
			return *result, errFolderFormat
		}

		result.conversion = conversion[0]
	}

	result.name = name

	return *result, nil
}

func (state *dateFormatState) replacement(field string, depth int) (string, error) {
	replacement, err := parseReplacement(field)
	if err != nil {
		return "", err
	}

	value, err := state.value(replacement.name)
	if err != nil {
		return "", err
	}

	spec, err := state.render(replacement.spec, depth+1)
	if err != nil {
		return "", err
	}

	if replacement.conversion != 0 {
		text := value.stringValue(replacement.conversion)

		return formatDateString(text, spec)
	}

	return value.format(spec)
}

func (state *dateFormatState) value(name string) (dateFormatValue, error) {
	parts := strings.Split(name, ".")
	if parts[0] == "" {
		if state.manual || state.automatic != 0 {
			return dateFormatValue{}, errFolderFormat
		}

		state.usedAutomatic = true
		state.automatic++
	} else {
		if state.usedAutomatic || parts[0] != "0" {
			return dateFormatValue{}, errFolderFormat
		}

		state.manual = true
	}

	value := new(dateFormatValue)
	value.instant = state.instant
	value.kind = dateObjectValue
	value.aware = true

	for _, attribute := range parts[1:] {
		updated, err := value.attribute(attribute)
		if err != nil {
			return *value, err
		}

		*value = updated
	}

	return *value, nil
}

type dateValueKind uint8

const (
	dateObjectValue dateValueKind = iota
	dateIntegerValue
	dateStringValue
	dateTimezoneValue
	dateDurationValue
)

type dateFormatValue struct {
	kind    dateValueKind
	instant time.Time
	integer int
	text    string
	aware   bool
}

func (value dateFormatValue) format(spec string) (string, error) {
	switch value.kind {
	case dateObjectValue:
		if spec == "" {
			return pythonDateString(value.instant, value.aware), nil
		}

		return formatDateAware(value.instant, spec, value.aware)
	case dateIntegerValue:
		return formatDateInteger(value.integer, spec)
	case dateStringValue:
		return formatDateString(value.text, spec)
	case dateTimezoneValue:
		if spec != "" {
			return "", errFolderFormat
		}

		if !value.aware {
			return "None", nil
		}

		return dateZone(value.instant, 'Z'), nil
	case dateDurationValue:
		if spec != "" {
			return "", errFolderFormat
		}

		return value.text, nil
	default:
		return "", errFolderFormat
	}
}

func (value dateFormatValue) stringValue(conversion byte) string {
	if conversion == 's' {
		text, _ := value.format("")

		return text
	}

	switch value.kind {
	case dateObjectValue:
		return pythonDateRepr(value.instant, value.aware)
	case dateTimezoneValue:
		if !value.aware {
			return "None"
		}

		return pythonTimezoneRepr(value.instant)
	case dateDurationValue:
		return "datetime.timedelta(microseconds=1)"
	case dateIntegerValue:
		return strconv.Itoa(value.integer)
	case dateStringValue:
		return "'" + strings.ReplaceAll(value.text, "'", "\\'") + "'"
	default:
		return ""
	}
}

func (value dateFormatValue) attribute(attribute string) (dateFormatValue, error) {
	if value.kind == dateIntegerValue {
		return value.integerAttribute(attribute)
	}

	if value.kind != dateObjectValue {
		return value, errFolderFormat
	}

	numbers := map[string]int{
		"year": value.instant.Year(), "month": int(value.instant.Month()), "day": value.instant.Day(),
		"hour": value.instant.Hour(), "minute": value.instant.Minute(), "second": value.instant.Second(),
		"microsecond": value.instant.Nanosecond() / dateMicrosecond, "fold": 0,
	}
	if number, ok := numbers[attribute]; ok {
		value.kind, value.integer = dateIntegerValue, number

		return value, nil
	}

	return value.dateAttribute(attribute)
}

func (value dateFormatValue) integerAttribute(attribute string) (dateFormatValue, error) {
	switch attribute {
	case "real", "numerator":
		return value, nil
	case "imag":
		value.integer = 0
	case "denominator":
		value.integer = 1
	default:
		return value, errFolderFormat
	}

	return value, nil
}

func (value dateFormatValue) dateAttribute(attribute string) (dateFormatValue, error) {
	switch attribute {
	case "tzinfo":
		value.kind = dateTimezoneValue
	case "min":
		value.instant = time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)
		value.aware = false
	case "max":
		value.instant = time.Date(9999, time.December, 31, 23, 59, 59, 999999000, time.UTC)
		value.aware = false
	case "resolution":
		value.kind, value.text = dateDurationValue, "0:00:00.000001"
	default:
		methods := "|date|time|timetz|astimezone|ctime|isocalendar|isoformat|isoweekday|replace|strftime|timestamp|" +
			"timetuple|timetz|toordinal|utcoffset|dst|tzname|utctimetuple|weekday|now|today|utcnow|" +
			"combine|fromtimestamp|utcfromtimestamp|fromordinal|fromisoformat|fromisocalendar|strptime|"
		if strings.Contains(methods, "|"+attribute+"|") {
			return value, unsupportedDateFormat("Python bound-method object address")
		}

		return value, fmt.Errorf("date attribute %q: %w", attribute, errFolderFormat)
	}

	return value, nil
}
