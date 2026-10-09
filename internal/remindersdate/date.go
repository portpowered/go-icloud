// Package remindersdate preserves the pinned Windows reference's UTC date conversion.
package remindersdate

import (
	"math"
	"time"
)

const (
	millisecondsPerSecond     = 1000
	microsecondsPerSecond     = 1000000
	nanosecondsPerMicrosecond = 1000
	// The pinned Python Windows runtime's gmtime range, verified at both boundaries.
	minimumUnixSecond int64 = -43200
	maximumUnixSecond int64 = 32536850399
)

// FromMillis follows datetime.fromtimestamp(int(value)/1000.0, UTC), including
// float64 seconds, rounding to microseconds and the pinned runtime's date bounds.
func FromMillis(millis int64) *time.Time {
	seconds, fraction := math.Modf(float64(millis) / millisecondsPerSecond)
	micros := int64(math.RoundToEven(fraction * microsecondsPerSecond))

	instant := time.Unix(int64(seconds), micros*nanosecondsPerMicrosecond).UTC()
	if instant.Unix() < minimumUnixSecond || instant.Unix() > maximumUnixSecond {
		return nil
	}

	return &instant
}
