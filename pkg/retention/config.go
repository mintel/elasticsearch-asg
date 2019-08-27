package retention

import "time"

// Config represents the number of snapshots to retain.
type Config struct {
	// Number of hourly snapshots to retain.
	Hourly uint

	// Number of daily snapshots to retain.
	Daily uint

	// Number of weekly snapshots to retain.
	Weekly uint

	// Number of monthly snapshots to retain.
	Monthly uint

	// Number of yearly snapshots to retain.
	Yearly uint
}

// MinInterval returns the minimum duration between snapshots.
// This is the interval between which backup jobs should run.
// If no buckets are defined, returns -1.
func (c Config) MinInterval() time.Duration {
	switch {
	case c.Hourly != 0:
		return Hour
	case c.Daily != 0:
		return Day
	case c.Weekly != 0:
		return Week
	case c.Monthly != 0:
		return Month
	case c.Yearly != 0:
		return Year
	}
	return -1
}

func (c Config) len() int {
	return int(c.Hourly + c.Daily + c.Weekly + c.Monthly + c.Yearly)
}
