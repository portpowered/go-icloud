package bridge

import (
	"slices"
	"strings"
)

type hostRuneRange struct {
	first, last rune
}

func lowercaseHostName(name string) string {
	letters := []rune(name)

	var output strings.Builder

	previousCased := false

	for index, value := range letters {
		if value == '\u03a3' && previousCased && !followingHostCased(letters[index+1:]) {
			output.WriteRune('\u03c2')
		} else if lowered, found := hostLowercase[value]; found {
			output.WriteString(lowered)
		} else {
			output.WriteRune(value)
		}

		if !hostRuneIn(hostCaseIgnored[:], value) {
			previousCased = hostRuneIn(hostCased[:], value)
		}
	}

	return output.String()
}

func followingHostCased(letters []rune) bool {
	for _, value := range letters {
		if !hostRuneIn(hostCaseIgnored[:], value) {
			return hostRuneIn(hostCased[:], value)
		}
	}

	return false
}

func hostRuneIn(ranges []hostRuneRange, value rune) bool {
	_, found := slices.BinarySearchFunc(ranges, value, func(interval hostRuneRange, selected rune) int {
		if selected < interval.first {
			return 1
		}

		if selected > interval.last {
			return -1
		}

		return 0
	})

	return found
}
