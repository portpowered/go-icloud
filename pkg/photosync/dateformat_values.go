package photosync

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func pythonDateString(instant time.Time, aware bool) string {
	result := instant.Format("2006-01-02 15:04:05")
	if microsecond := instant.Nanosecond() / dateMicrosecond; microsecond != 0 {
		result += fmt.Sprintf(".%06d", microsecond)
	}

	if aware {
		_, offset := instant.Zone()
		result += dateOffset(offset, true)
	}

	return result
}

func pythonDateRepr(instant time.Time, aware bool) string {
	parts := []int{instant.Year(), int(instant.Month()), instant.Day(), instant.Hour(), instant.Minute()}
	if instant.Second() != 0 || instant.Nanosecond()/dateMicrosecond != 0 {
		parts = append(parts, instant.Second())
	}

	if microsecond := instant.Nanosecond() / dateMicrosecond; microsecond != 0 {
		parts = append(parts, microsecond)
	}

	values := make([]string, len(parts))
	for index, number := range parts {
		values[index] = strconv.Itoa(number)
	}

	result := "datetime.datetime(" + strings.Join(values, ", ")
	if aware {
		result += ", tzinfo=" + pythonTimezoneRepr(instant)
	}

	return result + ")"
}

func pythonTimezoneRepr(instant time.Time) string {
	_, offset := instant.Zone()
	if offset == 0 {
		return "datetime.timezone.utc"
	}

	days := offset / dateSecondsDay
	seconds := offset % dateSecondsDay
	if seconds < 0 {
		days--
		seconds += dateSecondsDay
	}

	parts := []string{}
	if days != 0 {
		parts = append(parts, "days="+strconv.Itoa(days))
	}

	if seconds != 0 {
		parts = append(parts, "seconds="+strconv.Itoa(seconds))
	}

	return "datetime.timezone(datetime.timedelta(" + strings.Join(parts, ", ") + "))"
}

const (
	dateSecondsDay       = 86400
	dateMaximumWidth     = 1 << 20
	dateDefaultPrecision = 6
	dateBinaryBase       = 2
	dateOctalBase        = 8
	dateDecimalBase      = 10
	dateHexBase          = 16
)

type dateValueSpec struct {
	fill          string
	align         string
	sign          string
	normalizeZero bool
	sharp         bool
	zero          bool
	width         int
	group         string
	precision     int
	kind          string
}

func parseDateValueSpec(spec string) (dateValueSpec, error) {
	pattern := regexp.MustCompile(`^(?:(.)([<>=^])|([<>=^]))?([+ -])?(z)?(#)?(0)?` +
		`([0-9]*)([_,])?(?:\.([0-9]+))?([bcdeEfFgGnosxX%]?)$`)
	match := pattern.FindStringSubmatch(spec)
	result := new(dateValueSpec)
	result.precision = -1
	if match == nil {
		return *result, errFolderFormat
	}

	result.fill, result.align = match[1], match[2]+match[3]
	result.sign, result.normalizeZero = match[4], match[5] != ""
	result.sharp, result.zero = match[6] != "", match[7] != ""
	result.group, result.kind = match[9], match[11]
	result.width, _ = strconv.Atoi(match[8])
	if result.width > dateMaximumWidth {
		return *result, errFolderFormat
	}

	if match[10] != "" {
		result.precision, _ = strconv.Atoi(match[10])
		if result.precision > dateMaximumWidth {
			return *result, errFolderFormat
		}
	}

	return *result, nil
}

func formatDateString(value, spec string) (string, error) {
	options, err := parseDateValueSpec(spec)
	if err != nil || options.sign != "" || options.sharp || options.group != "" || options.normalizeZero ||
		options.align == "=" || options.kind != "" && options.kind != "s" {
		return "", errFolderFormat
	}

	if options.precision >= 0 {
		characters := []rune(value)
		if len(characters) > options.precision {
			value = string(characters[:options.precision])
		}
	}

	if options.align == "" {
		options.align = "<"
	}

	return padDateValue(value, options), nil
}

func formatDateInteger(value int, spec string) (string, error) {
	options, err := parseDateValueSpec(spec)
	if err != nil {
		return "", err
	}

	if strings.ContainsAny(options.kind, "eEfFgG%") && options.kind != "" {
		return formatDateFloat(value, options)
	}

	if options.precision >= 0 || options.normalizeZero {
		return "", errFolderFormat
	}

	text, err := dateIntegerText(value, options)
	if err != nil {
		return "", err
	}

	if options.sign == "+" || options.sign == " " {
		text = options.sign + text
	}

	if options.kind == "c" {
		return padDateValue(text, options), nil
	}

	base := dateDecimalBase

	switch options.kind {
	case "b":
		base = dateBinaryBase
	case "o":
		base = dateOctalBase
	case "x", "X":
		base = dateHexBase
	}

	return padDateNumber(text, options, base), nil
}

func dateIntegerText(value int, options dateValueSpec) (string, error) {
	bases := map[string]int{"": dateDecimalBase, "d": dateDecimalBase, "n": dateDecimalBase,
		"b": dateBinaryBase, "o": dateOctalBase, "x": dateHexBase, "X": dateHexBase}
	base, ok := bases[options.kind]
	if !ok {
		if options.kind == "c" && value >= 0 && value <= utf8.MaxRune &&
			options.sign == "" && !options.sharp && options.group == "" {
			return string(rune(value)), nil
		}

		return "", errFolderFormat
	}

	if options.group == "," && base != dateDecimalBase || options.group != "" && options.kind == "n" {
		return "", errFolderFormat
	}

	text := strconv.FormatInt(int64(value), base)

	if options.sharp && base != dateDecimalBase {
		prefixes := map[int]string{dateBinaryBase: "0b", dateOctalBase: "0o", dateHexBase: "0x"}
		text = prefixes[base] + text
	}

	if options.kind == "X" {
		text = strings.ToUpper(text)
	}

	return text, nil
}

func groupDateNumber(text, separator string, base int) string {
	if separator == "" {
		return text
	}

	group := 3
	if base != dateDecimalBase {
		group = 4
	}

	var output strings.Builder

	first := len(text) % group
	if first == 0 {
		first = group
	}

	output.WriteString(text[:first])

	for index := first; index < len(text); index += group {
		output.WriteString(separator)
		output.WriteString(text[index : index+group])
	}

	return output.String()
}

func formatDateFloat(value int, options dateValueSpec) (string, error) {
	kind := options.kind[0]
	number := float64(value)
	if kind == '%' {
		number *= dateYearCentury
		kind = 'f'
	}

	if kind == 'F' {
		kind = 'f'
	}

	precision := options.precision
	if precision < 0 {
		precision = dateDefaultPrecision
	}

	text := strconv.FormatFloat(number, kind, precision, 64)

	if options.sharp {
		format := "%#.*" + string(kind)
		text = fmt.Sprintf(format, precision, number)
	}

	if options.kind == "%" {
		text += "%"
	}

	if options.sign == "+" || options.sign == " " {
		text = options.sign + text
	}

	return padDateNumber(text, options, dateDecimalBase), nil
}

func padDateNumber(text string, options dateValueSpec, base int) string {
	if options.group == "" {
		return padDateValue(text, options)
	}

	prefixLength := dateNumberPrefix(text)
	prefix, rest := text[:prefixLength], text[prefixLength:]
	suffixIndex := -1
	if base == dateDecimalBase {
		suffixIndex = strings.IndexAny(rest, ".eE%")
	}

	digits, suffix := rest, ""
	if suffixIndex >= 0 {
		digits, suffix = rest[:suffixIndex], rest[suffixIndex:]
	}

	if (options.align == "=" || options.align == "" && options.zero) &&
		(options.fill == "0" || options.fill == "" && options.zero) {
		group := 3
		if base != dateDecimalBase {
			group = 4
		}

		desired := options.width - len(prefix) - len(suffix)
		required := desired - (desired-1)/(group+1)
		if required > len(digits) {
			digits = strings.Repeat("0", required-len(digits)) + digits
		}
	}

	text = prefix + groupDateNumber(digits, options.group, base) + suffix

	return padDateValue(text, options)
}

func dateNumberPrefix(text string) int {
	length := 0
	if strings.HasPrefix(text, "+") || strings.HasPrefix(text, " ") || strings.HasPrefix(text, "-") {
		length++
	}

	if len(text) >= length+2 && text[length] == '0' && strings.ContainsRune("boxBOX", rune(text[length+1])) {
		length += 2
	}

	return length
}

func padDateValue(text string, options dateValueSpec) string {
	padding := options.width - utf8.RuneCountInString(text)
	if padding <= 0 {
		return text
	}

	fill := options.fill
	if fill == "" {
		fill = " "
		if options.zero {
			fill = "0"
		}
	}

	align := options.align
	if align == "" {
		align = ">"
		if options.zero {
			align = "="
		}
	}

	return alignDateValue(text, fill, align, padding)
}

func alignDateValue(text, fill, align string, padding int) string {
	switch align {
	case "<":
		return text + strings.Repeat(fill, padding)
	case "^":
		return strings.Repeat(fill, padding/2) + text + strings.Repeat(fill, padding-padding/2)
	case "=":
		prefix := 0
		if strings.HasPrefix(text, "+") || strings.HasPrefix(text, " ") || strings.HasPrefix(text, "-") {
			prefix++
		}

		if len(text) >= prefix+2 && strings.ContainsRune("boxBOX", rune(text[prefix+1])) && text[prefix] == '0' {
			prefix += 2
		}

		return text[:prefix] + strings.Repeat(fill, padding) + text[prefix:]
	default:
		return strings.Repeat(fill, padding) + text
	}
}
