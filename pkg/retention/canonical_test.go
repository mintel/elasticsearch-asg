package retention

import (
	"time"
)

// canonical returns a canonical example of the inputs and outputs
// of Keep. It can be used for testing other components as well.
//
// times represts a set of input times to Keep.
//
// initialBuckets represents what buckets should look like before
// after newBuckets but before redistributeBackups is called.
//
// redistributedBuckets represents what buckets should look like after
// redistributeBackups is called.
//
// keep is what the output of Keep should be.
//
// del is what the output of Delete should be.
func canonical() (
	c Config,
	times []time.Time,
	initialBuckets buckets,
	redistributedBuckets buckets,
	keep []time.Time,
	del []time.Time,
) {
	// An example Config.
	c = Config{
		Hourly:  3,
		Daily:   2,
		Weekly:  3,
		Monthly: 1,
		Yearly:  1,
	}

	// Catchall bucket: 2014-01-04 17:26:42 => 2014-01-04 17:26:42
	catchallT0 := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)

	// Year 0 bucket: 2014-01-04 17:26:42 => 2015-01-04 23:15:54
	y0t0 := time.Date(2015, time.January, 1, 0, 0, 0, 0, time.UTC)

	// Month 0 bucket: 2015-01-04 23:15:54 => 2015-02-04 09:45:00
	m0t0 := time.Date(2015, time.January, 4, 23, 33, 0, 0, time.UTC)
	m0t1 := time.Date(2015, time.January, 5, 0, 0, 0, 0, time.UTC)
	m0t2 := time.Date(2015, time.January, 8, 0, 0, 0, 0, time.UTC)
	m0t3 := time.Date(2015, time.January, 16, 0, 0, 0, 0, time.UTC)
	m0t4 := time.Date(2015, time.January, 22, 0, 0, 0, 0, time.UTC)

	// Week 0 bucket: 2015-02-04 09:45:00 => 2015-02-11 09:45:00
	// w0t0 := time.Date(2015, time.February, 4, 23, 55, 0, 0, time.UTC)
	w0t1 := time.Date(2015, time.February, 7, 7, 21, 0, 0, time.UTC)

	// Week 1 bucket: 2015-02-11 09:45:00 => 2015-02-18 09:45:00
	w1t0 := time.Date(2015, time.February, 11, 12, 12, 0, 0, time.UTC)
	w1t1 := time.Date(2015, time.February, 12, 13, 13, 0, 0, time.UTC)
	w1t2 := time.Date(2015, time.February, 15, 22, 14, 0, 0, time.UTC)
	w1t3 := time.Date(2015, time.February, 17, 18, 15, 0, 0, time.UTC)
	w1t4 := time.Date(2015, time.February, 18, 9, 40, 0, 0, time.UTC)

	// Week 2 bucket: 2015-02-18 09:45:00 => 2015-02-25 09:45:00

	// Day 0 bucket: 2015-02-25 09:45:00 => 2015-02-26 09:45:00

	// Day 1 bucket: 2015-02-26 09:45:00 => 2015-02-27 09:45:00
	d1t0 := time.Date(2015, time.February, 26, 9, 50, 0, 0, time.UTC)
	d1t1 := time.Date(2015, time.February, 26, 22, 30, 0, 0, time.UTC)
	d1t2 := time.Date(2015, time.February, 27, 1, 1, 0, 0, time.UTC)

	// Hour 0 bucket: 2015-02-27 09:45:00 => 2015-02-27 10:45:00
	h0t0 := time.Date(2015, time.February, 27, 10, 1, 0, 0, time.UTC)
	h0t1 := time.Date(2015, time.February, 27, 10, 29, 0, 0, time.UTC)

	// Hour 1 bucket: 2015-02-27 10:45:00 => 2015-02-27 11:45:00
	h1t0 := time.Date(2015, time.February, 27, 11, 05, 0, 0, time.UTC)
	h1t1 := time.Date(2015, time.February, 27, 11, 28, 0, 0, time.UTC)

	// Hour 2 bucket: 2015-02-27 11:45:00 => 2015-02-27 12:45:00
	// The last time defines all bucket boundaries.
	h2t0 := time.Date(2015, time.February, 27, 12, 2, 0, 0, time.UTC)
	h2t1 := time.Date(2015, time.February, 27, 12, 20, 0, 0, time.UTC)
	h2t2 := time.Date(2015, time.February, 27, 12, 45, 0, 0, time.UTC)

	times = []time.Time{
		catchallT0,
		y0t0,
		m0t0, m0t1, m0t2, m0t3, m0t4,
		w0t1,
		w1t0, w1t1, w1t2, w1t3, w1t4,
		d1t0, d1t1, d1t2,
		h0t0, h0t1,
		h1t0, h1t1,
		h2t0, h2t1, h2t2,
	}

	initialBuckets = buckets{
		&bucket{
			// Start: time.Date(2014, time.January, 4, 17, 26, 42, 0, time.UTC),
			End:   time.Date(2014, time.January, 4, 17, 26, 42, 0, time.UTC),
			Width: 0,
			Snapshots: timeseries{
				catchallT0,
			},
		},
		&bucket{
			// Start: time.Date(2014, time.January, 4, 17, 26, 42, 0, time.UTC),
			End:   time.Date(2015, time.January, 4, 23, 15, 54, 0, time.UTC),
			Width: Year,
			Snapshots: timeseries{
				y0t0, // Kept
			},
		},
		&bucket{
			// Start: time.Date(2015, time.January, 4, 23, 15, 54, 0, time.UTC),
			End:   time.Date(2015, time.February, 4, 9, 45, 0, 0, time.UTC),
			Width: Month,
			Snapshots: timeseries{
				m0t0, // Kept
				m0t1,
				m0t2,
				m0t3,
				m0t4, // Kept (interval algorithm)
			},
		},
		&bucket{
			// Start: time.Date(2015, time.February, 4, 9, 45, 0, 0, time.UTC),
			End:   time.Date(2015, time.February, 11, 9, 45, 0, 0, time.UTC),
			Width: Week,
			Snapshots: timeseries{
				w0t1, // Kept
			},
		},
		&bucket{
			// Start: time.Date(2015, time.February, 11, 9, 45, 0, 0, time.UTC),
			End:   time.Date(2015, time.February, 18, 9, 45, 0, 0, time.UTC),
			Width: Week,
			Snapshots: timeseries{
				w1t0, // Kept
				w1t1,
				w1t2,
				w1t3,
				w1t4, // Kept (will be loaned to next bucket)
			},
		},
		&bucket{
			// Start: time.Date(2015, time.February, 18, 9, 45, 0, 0, time.UTC),
			End:       time.Date(2015, time.February, 25, 9, 45, 0, 0, time.UTC),
			Width:     Week,
			Snapshots: timeseries(nil),
		},
		&bucket{
			// Start: time.Date(2015, time.February, 25, 9, 45, 0, 0, time.UTC),
			End:       time.Date(2015, time.February, 26, 9, 45, 0, 0, time.UTC),
			Width:     Day,
			Snapshots: timeseries(nil),
		},
		&bucket{
			// Start: time.Date(2015, time.February, 26, 9, 45, 0, 0, time.UTC),
			End:   time.Date(2015, time.February, 27, 9, 45, 0, 0, time.UTC),
			Width: Day,
			Snapshots: timeseries{
				d1t0, // Kept (will be borrowed by previous bucket)
				d1t1, // Kept
				d1t2, // Kept (interval algorithm)
			},
		},
		&bucket{
			// Start: time.Date(2015, time.February, 27, 9, 45, 0, 0, time.UTC),
			End:   time.Date(2015, time.February, 27, 10, 45, 0, 0, time.UTC),
			Width: Hour,
			Snapshots: timeseries{
				h0t0, // Kept
				h0t1, // Kept
			},
		},
		&bucket{
			// Start: time.Date(2015, time.February, 27, 10, 45, 0, 0, time.UTC),
			End:   time.Date(2015, time.February, 27, 11, 45, 0, 0, time.UTC),
			Width: Hour,
			Snapshots: timeseries{
				h1t0, // Kept
				h1t1, // Kept
			},
		},
		&bucket{
			// Start: time.Date(2015, time.February, 27, 11, 45, 0, 0, time.UTC),
			End:   time.Date(2015, time.February, 27, 12, 45, 0, 0, time.UTC),
			Width: Hour,
			Snapshots: timeseries{
				h2t0, // Kept
				h2t1, // Kept
				h2t2, // Kept
			},
		},
	}

	redistributedBuckets = buckets{
		&bucket{
			// Start: time.Date(2014, time.January, 4, 17, 26, 42, 0, time.UTC),
			End:   time.Date(2014, time.January, 4, 17, 26, 42, 0, time.UTC),
			Width: 0,
			Snapshots: timeseries{
				catchallT0,
			},
		},
		&bucket{
			// Start: time.Date(2014, time.January, 4, 17, 26, 42, 0, time.UTC),
			End:   time.Date(2015, time.January, 4, 23, 15, 54, 0, time.UTC),
			Width: Year,
			Snapshots: timeseries{
				y0t0, // Kept
			},
		},
		&bucket{
			// Start: time.Date(2015, time.January, 4, 23, 15, 54, 0, time.UTC),
			End:   time.Date(2015, time.February, 4, 9, 45, 0, 0, time.UTC),
			Width: Month,
			Snapshots: timeseries{
				m0t0, // Kept
				m0t1,
				m0t2,
				m0t3,
				m0t4, // Kept
			},
		},
		&bucket{
			// Start: time.Date(2015, time.February, 4, 9, 45, 0, 0, time.UTC),
			End:   time.Date(2015, time.February, 11, 9, 45, 0, 0, time.UTC),
			Width: Week,
			Snapshots: timeseries{
				w0t1, // Kept
			},
		},
		&bucket{
			// Start: time.Date(2015, time.February, 11, 9, 45, 0, 0, time.UTC),
			End:   time.Date(2015, time.February, 18, 9, 45, 0, 0, time.UTC),
			Width: Week,
			Snapshots: timeseries{
				w1t0, // Kept
				w1t1,
				w1t2,
				w1t3,
			},
		},
		&bucket{
			// Start: time.Date(2015, time.February, 18, 9, 45, 0, 0, time.UTC),
			End:   time.Date(2015, time.February, 25, 9, 45, 0, 0, time.UTC),
			Width: Week,
			Snapshots: timeseries{
				w1t4, // Kept (loaned from the previous bucket)
			},
		},
		&bucket{
			// Start: time.Date(2015, time.February, 25, 9, 45, 0, 0, time.UTC),
			End:   time.Date(2015, time.February, 26, 9, 45, 0, 0, time.UTC),
			Width: Day,
			Snapshots: timeseries{
				d1t0, // Kept (borrowed from the next bucket)
			},
		},
		&bucket{
			// Start: time.Date(2015, time.February, 26, 9, 45, 0, 0, time.UTC),
			End:   time.Date(2015, time.February, 27, 9, 45, 0, 0, time.UTC),
			Width: Day,
			Snapshots: timeseries{
				d1t1, // Kept
				d1t2,
			},
		},
		&bucket{
			// Start: time.Date(2015, time.February, 27, 9, 45, 0, 0, time.UTC),
			End:   time.Date(2015, time.February, 27, 10, 45, 0, 0, time.UTC),
			Width: Hour,
			Snapshots: timeseries{
				h0t0, // Kept
				h0t1, // Kept
			},
		},
		&bucket{
			// Start: time.Date(2015, time.February, 27, 10, 45, 0, 0, time.UTC),
			End:   time.Date(2015, time.February, 27, 11, 45, 0, 0, time.UTC),
			Width: Hour,
			Snapshots: timeseries{
				h1t0, // Kept
				h1t1, // Kept
			},
		},
		&bucket{
			// Start: time.Date(2015, time.February, 27, 11, 45, 0, 0, time.UTC),
			End:   time.Date(2015, time.February, 27, 12, 45, 0, 0, time.UTC),
			Width: Hour,
			Snapshots: timeseries{
				h2t0, // Kept
				h2t1, // Kept
				h2t2, // Kept
			},
		},
	}

	keep = []time.Time{
		y0t0,
		m0t0,
		m0t4,
		w0t1,
		w1t0, w1t4,
		d1t0, d1t1, d1t2,
		h0t0, h0t1,
		h1t0, h1t1,
		h2t0, h2t1, h2t2,
	}

	del = []time.Time{
		catchallT0,
		m0t1, m0t2, m0t3,
		w1t1, w1t2, w1t3,
	}

	return
}
