package time

import "time"

// Between returns true if t is between a and b inclusive.
func Between(t, a, b time.Time) bool {
	if b.Before(a) {
		a, b = b, a
	}
	return (a.Equal(t) || a.Before(t)) && (b.Equal(t) || b.After(t))
}
