package time

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/JohnCGriffin/overflow"
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
)

const iso8601Group = `(?:(?P<%s>-?\d+(?:[,.]\d+)?)%s)?`

var iso8601Duation = regexp.MustCompile(fmt.Sprintf(`^P(?:0|%s|%s)$`,
	fmt.Sprintf(iso8601Group, "weeks", "W"),
	fmt.Sprintf(`%s%s%s(?:T%s%s%s)?`,
		fmt.Sprintf(iso8601Group, "years", "Y"),
		fmt.Sprintf(iso8601Group, "months", "M"),
		fmt.Sprintf(iso8601Group, "days", "D"),
		fmt.Sprintf(iso8601Group, "hours", "H"),
		fmt.Sprintf(iso8601Group, "minutes", "M"),
		fmt.Sprintf(iso8601Group, "seconds", "S"),
	),
))

// ParseISO8601D parses an ISO 8601 duration string.
// Errors are returned if the string can't be parsed or the duration
// overflows int64.
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
		return 0, fmt.Errorf("cannot parse a blank string as a period")
	}

	if duration == "P0" {
		return 0, nil
	}

	matches := iso8601Duation.FindStringSubmatch(duration)
	if matches == nil {
		return 0, fmt.Errorf("cannot parse Period string")
	}
	groupNames := iso8601Duation.SubexpNames()
	var d time.Duration
	for i := 1; i < len(groupNames); i++ {
		group, match := groupNames[i], matches[i]
		if match == "" {
			continue
		}
		n, err := strconv.ParseFloat(strings.Replace(match, ",", ".", 1), 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse %s value '%s': %v", group, match, err)
		}
		var r time.Duration
		var ok bool
		switch group {
		case "weeks":
			r, ok = addDurationMul(d, n, Week)
		case "years":
			r, ok = addDurationMul(d, n, Year)
		case "months":
			r, ok = addDurationMul(d, n, Month)
		case "days":
			r, ok = addDurationMul(d, n, Day)
		case "hours":
			r, ok = addDurationMul(d, n, time.Hour)
		case "minutes":
			r, ok = addDurationMul(d, n, time.Minute)
		case "seconds":
			r, ok = addDurationMul(d, n, time.Second)
		}
		if !ok {
			return r, fmt.Errorf("int64 overflow")
		}
		d = r
	}
	return d, nil
}

// MustParseISO8601D is like ParseISO8601D, but panics if there's an error.
func MustParseISO8601D(duration string) time.Duration {
	d, err := ParseISO8601D(duration)
	if err != nil {
		panic(err)
	}
	return d
}

// addDurationMul returns d+n*u. It also returns a bool indicating if the result is valid (true)
// or it hit an int64 overflow (false).
func addDurationMul(d time.Duration, n float64, u time.Duration) (time.Duration, bool) {
	r := n * float64(u)
	if r > float64(math.MaxInt64) {
		return time.Duration(math.MaxInt64), false
	} else if r < float64(math.MinInt64) {
		return time.Duration(math.MinInt64), false
	}
	i, ok := overflow.Add64(int64(d), int64(r))
	return time.Duration(i), ok
}
