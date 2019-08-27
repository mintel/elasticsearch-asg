package retention

import (
	"time"
)

// Durations like day, week, month, and year are hard
// to specify due to the vagaries of calenders, leap seconds,
// the changing shape of the earth, planetary orbits, etc.
// Which is why they aren't specified in the "time" package.
// We're going to make some assumptions about those durations
// here because we aren't launching rockets or whatever.

const (
	// Hour duration.
	Hour = time.Hour

	// Day duration.
	Day = 24 * Hour

	// Week duration.
	Week = 7 * Day

	// Year duration.
	Year = time.Duration(365.2425 * float64(Day))

	// Month duration.
	Month = Year / 12
)
