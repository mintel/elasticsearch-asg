package retention

import (
	"sort"
	"time"
)

// timeseries is a unique, sorted slice of Times representing
// when snapshots were taken. It should be constructed
// using make() followed by it's Push() method.
type timeseries []time.Time

// Implement sort.Interface:

func (s timeseries) Len() int           { return len(s) }
func (s timeseries) Less(i, j int) bool { return s[i].Before(s[j]) }
func (s timeseries) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// Push inserts a one or more new snapshot Times in sorted order.
func (sp *timeseries) Push(times ...time.Time) {
	if len(times) == 0 {
		return
	}
	sortedTimes := timeseries(times)
	sortedTimes.squash()
	s := *sp
	if len(s) == 0 {
		*sp = sortedTimes
		return
	}
	s = append(s, sortedTimes...)
	s.squash()
	*sp = s
}

// Pop removes the time at an index are returns it.
// It panics if the index is not present.
func (s *timeseries) Pop(idx int) time.Time {
	t := (*s)[idx]
	if idx == 0 {
		*s = (*s)[1:]
		return t
	}
	if n := len(*s); idx == n-1 {
		*s = (*s)[:n-1]
		return t
	}
	*s = append((*s)[:idx], (*s)[idx+1:]...)
	return t
}

// Discard removes the specified snapshot Times from the
// timeseries if they are present.
func (s *timeseries) Discard(ts ...time.Time) {
	for _, t := range ts {
		if i := s.Find(t); i != -1 {
			_ = s.Pop(i)
		}
	}
}

// PopOldest pops the oldest snapshot off the slice
// and returns it. If the slice is empty it returns
// the zero Time.
func (s *timeseries) PopOldest() time.Time {
	if len(*s) == 0 {
		return time.Time{}
	}
	return s.Pop(0)
}

// PeekOldest returns the oldest snapshot in the slice.
// If the slice is empty it returns the zero Time.
func (s timeseries) PeekOldest() time.Time {
	if len(s) == 0 {
		return time.Time{}
	}
	return s[0]
}

// PopNewest pops the newest snapshot off the slice
// and returns it. If the slice is empty it returns
// the zero Time.
func (s *timeseries) PopNewest() time.Time {
	n := len(*s)
	if n == 0 {
		return time.Time{}
	}
	return s.Pop(n - 1)
}

// PeekNewest returns the newest snapshot in the slice.
// If the slice is empty it returns the zero Time.
func (s timeseries) PeekNewest() time.Time {
	n := len(s)
	if n == 0 {
		return time.Time{}
	}
	return s[n-1]
}

// Find returns the index of a snapshot is in the slice,
// or -1 if it is not found.
func (s timeseries) Find(t time.Time) int {
	i := sort.Search(len(s), func(i int) bool {
		return !s[i].Before(t)
	})
	if i < len(s) && s[i].Equal(t) {
		return i
	}
	return -1
}

// squash ensures the timeseries is sorted and unique.
func (sp *timeseries) squash() {
	s := *sp

	sort.Sort(s)

	// Make unique
	for i := 1; i < len(s)-1; i++ {
		t := s[i]
		if t.Equal(s[i+1]) {
			// Identify the slice of items that
			// are non-unique.
			start := i + 1
			end := start + 1
			for s[end].Equal(t) {
				end++
			}

			// All remaining items are non-unique.
			if end == len(s) {
				s = s[:start]
				break
			}

			copy(s[start:], s[end:])
			n := start + (len(s) - end)
			s = s[:n]
		}
	}

	*sp = s
}
