package photosync

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	dateMicrosecond  = 1000
	dateWeekDays     = 7
	dateHalfDayHours = 12
	dateYearCentury  = 100
)

func formatFolder(instant time.Time, format string) (string, error) {
	if strings.Contains(format, "{") {
		state := new(dateFormatState)
		state.instant = instant

		return state.render(format, 0)
	}

	return formatDate(instant, format)
}

func formatDate(instant time.Time, format string) (string, error) {
	return formatDateAware(instant, format, true)
}

func formatDateAware(instant time.Time, format string, aware bool) (string, error) {
	var output strings.Builder

	for index := 0; index < len(format); index++ {
		if format[index] != '%' {
			output.WriteByte(format[index])

			continue
		}

		index++
		if index == len(format) {
			return "", errFolderFormat
		}

		short := format[index] == '#'
		if short {
			index++
			if index == len(format) {
				return "", errFolderFormat
			}
		}

		alternate := format[index] == 'E' || format[index] == 'O'
		if alternate {
			index++
			if index == len(format) {
				return "", errFolderFormat
			}

			if format[index] == 'f' {
				return "", errFolderFormat
			}

			if format[index] == 'z' || format[index] == 'Z' {
				return "", unsupportedDateFormat("machine-local Windows CRT timezone")
			}
		}

		value, err := dateDirective(instant, format[index], short)
		if !aware && !short && (format[index] == 'z' || format[index] == 'Z') {
			value = ""
		}

		if err != nil {
			return "", err
		}

		output.WriteString(value)
	}

	return output.String(), nil
}

func dateDirective(instant time.Time, directive byte, short bool) (string, error) {
	if value, width, ok := dateNumber(instant, directive); ok {
		if short {
			return strconv.Itoa(value), nil
		}

		return fmt.Sprintf("%0*d", width, value), nil
	}

	if directive == 'f' {
		if short {
			return "", errFolderFormat
		}

		return fmt.Sprintf("%06d", instant.Nanosecond()/dateMicrosecond), nil
	}

	if directive == 'z' || directive == 'Z' {
		if short {
			return "", unsupportedDateFormat("machine-local Windows CRT timezone")
		}

		return dateZone(instant, directive), nil
	}

	return dateText(instant, directive, short)
}

func dateNumber(instant time.Time, directive byte) (int, int, bool) {
	isoYear, isoWeek := instant.ISOWeek()
	numbers := map[byte]int{
		'Y': instant.Year(), 'y': instant.Year() % dateYearCentury, 'm': int(instant.Month()),
		'C': instant.Year() / dateYearCentury,
		'd': instant.Day(), 'H': instant.Hour(), 'I': (instant.Hour()+dateHalfDayHours-1)%dateHalfDayHours + 1,
		'M': instant.Minute(), 'S': instant.Second(), 'j': instant.YearDay(), 'w': int(instant.Weekday()),
		'u': (int(instant.Weekday())+dateWeekDays-1)%dateWeekDays + 1, 'V': isoWeek,
		'G': isoYear, 'g': isoYear % dateYearCentury,
		'U': (instant.YearDay() + dateWeekDays - 1 - int(instant.Weekday())) / dateWeekDays,
		'W': (instant.YearDay() + dateWeekDays - 1 - (int(instant.Weekday())+dateWeekDays-1)%dateWeekDays) / dateWeekDays,
	}
	widths := map[byte]int{'Y': 4, 'G': 4, 'j': 3, 'w': 1, 'u': 1}
	value, recognized := numbers[directive]

	width := widths[directive]
	if width == 0 {
		width = 2
	}

	return value, width, recognized
}

func dateText(instant time.Time, directive byte, short bool) (string, error) {
	layouts := map[byte]string{
		'a': "Mon", 'A': "Monday", 'b': "Jan", 'h': "Jan", 'B': "January", 'p': "PM",
		'c': "Mon Jan _2 15:04:05 2006", 'x': "01/02/06", 'X': "15:04:05", 'e': "_2",
		'D': "01/02/06", 'F': "2006-01-02", 'r': "03:04:05 PM", 'R': "15:04", 'T': "15:04:05",
	}
	shortLayouts := map[byte]string{
		'c': "Monday, January 02, 2006 15:04:05", 'x': "Monday, January 02, 2006", 'e': "2",
		'D': "1/2/06", 'F': "2006-1-2", 'r': "3:4:5 PM", 'R': "15:4", 'T': "15:4:5",
	}

	if short {
		if layout, ok := shortLayouts[directive]; ok {
			return shortDateText(instant, directive, layout), nil
		}
	}

	if layout, ok := layouts[directive]; ok {
		return instant.Format(layout), nil
	}

	switch directive {
	case '%':
		return "%", nil
	case 'n':
		return "\n", nil
	case 't':
		return "\t", nil
	default:
		return "", errFolderFormat
	}
}

func shortDateText(instant time.Time, directive byte, layout string) string {
	if directive == 'R' || directive == 'T' {
		text := strconv.Itoa(instant.Hour()) + ":" + strconv.Itoa(instant.Minute())
		if directive == 'T' {
			text += ":" + strconv.Itoa(instant.Second())
		}

		return text
	}

	if directive == 'r' {
		hour := (instant.Hour()+dateHalfDayHours-1)%dateHalfDayHours + 1

		return fmt.Sprintf("%d:%d:%d %s", hour, instant.Minute(), instant.Second(), instant.Format("PM"))
	}

	return instant.Format(layout)
}

func dateZone(instant time.Time, directive byte) string {
	name, offset := instant.Zone()
	if directive == 'Z' {
		if offset == 0 {
			return "UTC"
		}

		if name == "" {
			return "UTC" + dateOffset(offset, true)
		}

		return name
	}

	return dateOffset(offset, false)
}

const (
	dateSecondsMinute = 60
	dateSecondsHour   = 3600
)

func dateOffset(offset int, colon bool) string {
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}

	separator := ""
	if colon {
		separator = ":"
	}

	result := fmt.Sprintf("%s%02d%s%02d", sign, offset/dateSecondsHour,
		separator, offset/dateSecondsMinute%dateSecondsMinute)
	if offset%dateSecondsMinute != 0 {
		result += fmt.Sprintf("%s%02d", separator, offset%dateSecondsMinute)
	}

	return result
}
