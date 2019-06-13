package time

import "time"

// Ceil returns the result of rounding t up to a multiple of d (since the zero time).
// If d <= 0, Ceil returns t unchanged.
func Ceil(t time.Time, d time.Duration) time.Time {
	if d < 0 {
		return t
	}
	return t.Add(d).Truncate(d)
}

// Prev returns the nearest multiple of d before t (since the zero time).
// If d <= 0, Prev returns t unchanged.
func Prev(t time.Time, d time.Duration) time.Time {
	if d < 0 {
		return t
	}
	t2 := t.Truncate(d)
	if t2.Equal(t) {
		t2 = t2.Add(-d)
	}
	return t2
}

// Next returns the nearest multiple of d after t (since the zero time).
// If d <= 0, Next returns t unchanged.
func Next(t time.Time, d time.Duration) time.Time {
	if d < 0 {
		return t
	}
	return t.Truncate(d).Add(d)
}

// IsMultiple returns true if t is some multiple of d (since the zero time).
// If d <= 0, IsMultiple returns false.
func IsMultiple(t time.Time, d time.Duration) bool {
	if d < 0 {
		return false
	}
	return t.Truncate(d).Equal(t)
}
