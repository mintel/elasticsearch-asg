package time

import "time"

// Between returns true if t is between start and end inclusive.
func Between(t, start, end time.Time) bool {
	return (start.Equal(t) || start.Before(t)) && (end.Equal(t) || end.After(t))
}
