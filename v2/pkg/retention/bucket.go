package retention

import (
	"sort"
	"time"
)

// bucket represents a time window to which
// snapshots are assigned.
type bucket struct {
	// The size of the bucket.
	Width time.Duration

	// The point in time at which the bucket ends.
	// If you plot time as a line with oldest on the left
	// and newest on the right, this is the right-most edge
	// of the bucket.
	End time.Time

	// The snapshots assigned to this bucket.
	// Note that the snapshots do not
	// necessarily need to fall between the start
	// and end of the bucket. Rather, the purpose of
	// a bucket is to assist with distributing
	// snapshots approximately evenly across time
	// by reassigning them to neighboring buckets.
	Snapshots timeseries
}

// Start returns the start time of the bucket.
func (b *bucket) Start() time.Time {
	return b.End.Add(-b.Width)
}

// IsCatchall returns true if this is the
// catchall bucket that holds any snapshot times
// not in other buckets.
func (b *bucket) IsCatchall() bool {
	return b.Width == 0
}

// buckets is a sorted slice of buckets.
type buckets []*bucket

// newBuckets returns a buckets slice based on a
// Config and the time of the latest snapshot. The
// slice is populated with a number of Buckets defined
// by the Config, plus one more catchall bucket to
// hold snapshots that don't fit anywhere else.
// The catchall bucket will be at index 0, the
// oldest and largest Bucket will be at index 1,
// and the last Bucket in the slice will
// be the smallest and End at Time T.
func newBuckets(c Config, end time.Time) buckets {
	n := c.len()
	if n == 0 {
		return nil
	}
	out := make(buckets, n+1)

	for i := n; i >= 1; i-- {
		w := c.MinInterval()

		// c is a copy of the config,
		// so we can decrement values without
		// a problem.
		switch w {
		case Hour:
			c.Hourly--
		case Day:
			c.Daily--
		case Week:
			c.Weekly--
		case Month:
			c.Monthly--
		case Year:
			c.Yearly--
		case -1:
			panic("ran out of buckets")
		default:
			panic("unknown interval")
		}

		out[i] = &bucket{
			Width: w,
			End:   end,
		}
		end = end.Add(-w)
	}

	// Catchall bucket.
	out[0] = &bucket{
		End:   end,
		Width: 0,
	}

	return out
}

// Implement sort.Interface:

func (bs buckets) Len() int { return len(bs) }
func (bs buckets) Less(i, j int) bool {
	bI, bJ := bs[i], bs[j]
	if bI.Width > bJ.Width {
		return true
	} else if bI.Width < bJ.Width {
		return false
	}
	return bI.End.Before(bJ.End)
}
func (bs buckets) Swap(i, j int) { bs[i], bs[j] = bs[j], bs[i] }

// For returns the bucket for which t falls
// between the start and end of the bucket.
// Returns nil if such a bucket doesn't exist.
func (bs buckets) For(t time.Time) *bucket {
	idx := sort.Search(len(bs), func(i int) bool {
		return !bs[i].End.Before(t)
	})
	if idx == len(bs) {
		return nil
	}
	if idx == 0 && len(bs) > 1 && t.Equal(bs[0].End) {
		return bs[1]
	}
	return bs[idx]
}

// Has returns the bucket that actually contains
// a given snapshot.
func (bs buckets) Has(snapshot time.Time) *bucket {
	for _, b := range bs {
		if b.Snapshots.Find(snapshot) != -1 {
			return b
		}
	}
	return nil
}

// Assign assigns each time in a timeseries to a bucket.
// Will panic if one of the times doesn't fit in a bucket.
func (bs buckets) Assign(ts timeseries) {
	for _, t := range ts {
		b := bs.For(t)
		b.Snapshots.Push(t)
	}
}

// Start returns the oldest-most boundary of all buckets.
// Returns the zero time if buckets is empty.
func (bs buckets) Start() time.Time {
	if len(bs) == 0 {
		return time.Time{}
	}
	return bs[0].Start()
}

// End returns the newest-most boundary of all buckets.
// Returns the zero time if buckets is empty.
func (bs buckets) End() time.Time {
	if len(bs) == 0 {
		return time.Time{}
	}
	return bs[len(bs)-1].End
}
