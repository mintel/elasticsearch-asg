package time

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
)

const (
	// Day duration
	Day = 24 * time.Hour

	// Week duration
	Week = 7 * Day

	// Month duration
	Month = (time.Duration(30.436875*float64(Day)) / time.Second) * time.Second // truncate to second

	// Year duration
	Year = (time.Duration(365.2425*float64(Day)) / time.Second) * time.Second // truncate to second

	iso8601Group        = `(?:(?P<%s>-?\d+(?:[,.]\d+)?)%s)?`
	iso8601GroupWeeks   = "W"
	iso8601GroupYears   = "Y"
	iso8601GroupMonths  = "m"
	iso8601GroupDays    = "d"
	iso8601GroupHours   = "H"
	iso8601GroupMinutes = "M"
	iso8601GroupSeconds = "S"
)

var iso8601Duration = regexp.MustCompile(fmt.Sprintf(`^P(?:0|%s|%s)$`,
	fmt.Sprintf(iso8601Group, iso8601GroupWeeks, "W"),
	fmt.Sprintf(`%s%s%s(?:T%s%s%s)?`,
		fmt.Sprintf(iso8601Group, iso8601GroupYears, "Y"),
		fmt.Sprintf(iso8601Group, iso8601GroupMonths, "M"),
		fmt.Sprintf(iso8601Group, iso8601GroupDays, "D"),
		fmt.Sprintf(iso8601Group, iso8601GroupHours, "H"),
		fmt.Sprintf(iso8601Group, iso8601GroupMinutes, "M"),
		fmt.Sprintf(iso8601Group, iso8601GroupSeconds, "S"),
	),
))

// ErrInt64Overflow is returned by ParseISO8601D if the duration can't fit in an int64.
var ErrInt64Overflow = errors.New("int64 overflow")

// ParseISO8601D parses an ISO 8601 duration string.
// Errors are returned if the string can't be parsed or the duration
// overflows an int64.
//
// The following time assumptions are used:
// - Days are 24 hours.
// - Weeks are 7 days.
// - Months are 30.436875 days.
// - Years are 365.2425 days.
//
// See: https://en.wikipedia.org/wiki/ISO_8601#Durations
func ParseISO8601D(duration string) (time.Duration, error) {
	if duration == "" {
		return 0, fmt.Errorf("cannot parse a blank string as a duration")
	}
	if duration == "P0" {
		return 0, nil
	}
	matches := iso8601Duration.FindStringSubmatch(duration)
	if matches == nil {
		return 0, fmt.Errorf("cannot parse duration string")
	}
	groupNames := iso8601Duration.SubexpNames()
	const precision = 128 // Arbitrary number bigger than an int64
	sum := new(big.Float).SetPrec(precision)
	for i := 1; i < len(groupNames); i++ {
		group, match := groupNames[i], matches[i]
		if match == "" {
			continue
		}
		n, _, err := big.ParseFloat(
			strings.Replace(match, ",", ".", 1), // Convert comma decimal separator to period.
			10,
			precision,
			big.ToZero,
		)
		if err != nil {
			return 0, fmt.Errorf("failed to parse %s value '%s': %v", group, match, err)
		}
		var d time.Duration
		switch group {
		case iso8601GroupWeeks:
			d = Week
		case iso8601GroupYears:
			d = Year
		case iso8601GroupMonths:
			d = Month
		case iso8601GroupDays:
			d = Day
		case iso8601GroupHours:
			d = time.Hour
		case iso8601GroupMinutes:
			d = time.Minute
		case iso8601GroupSeconds:
			d = time.Second
		}
		bigD := new(big.Float).SetInt64(int64(d))
		n = n.Mul(n, bigD)
		sum = sum.Add(sum, n)
	}
	switch i, a := sum.Int64(); a {
	case big.Below, big.Above:
		return time.Duration(i), ErrInt64Overflow
	case big.Exact:
		return time.Duration(i), nil
	default:
		panic("unknown accuracy: " + a.String())
	}
}

// MustParseISO8601D is like ParseISO8601D, but panics if there's an error.
func MustParseISO8601D(duration string) time.Duration {
	d, err := ParseISO8601D(duration)
	if err != nil {
		panic(err)
	}
	return d
}
