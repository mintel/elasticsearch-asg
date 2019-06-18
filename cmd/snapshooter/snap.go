package main

import (
	"time"

	ptime "github.com/mintel/elasticsearch-asg/pkg/time"
)

// SnapshotWindow represents how often to take Elasticsearch snapshots,
// and how long to keep them.
type SnapshotWindow struct {
	Every   time.Duration // Take snapshots with this frequency.
	KeepFor time.Duration // Keep snapshots for this long.
}

// NewSnapshotWindow returns a new SnapshotWindow by parsing two ISO 8601 Duration strings.
func NewSnapshotWindow(every, keepFor string) (SnapshotWindow, error) {
	sw := SnapshotWindow{}

	d, err := ptime.ParseISO8601D(every)
	if err != nil {
		return SnapshotWindow{}, err
	}
	sw.Every = d

	d, err = ptime.ParseISO8601D(keepFor)
	if err != nil {
		return SnapshotWindow{}, err
	}
	sw.KeepFor = d

	return sw, nil
}

// SnapshotWindows is a slice that can be used to
// determine when the next Elasticsearch snapshot should be taken,
// and when an old snapshot should be deleted.
type SnapshotWindows []SnapshotWindow

// Next retuns the next Time a snapshot should be created.
// If this SnapshotWindows is empty, returns the zero Time.
func (sws SnapshotWindows) Next() time.Time {
	// Only one snapshot can be creating at the same time.
	// TODO: Track past snapshot durations, predict future durations, and choose times that don't clobber each other.
	return sws.next(time.Now().UTC())
}

func (sws SnapshotWindows) next(now time.Time) time.Time {
	var t time.Time
	for _, sw := range sws {
		if t2 := ptime.Next(now, sw.Every); t.IsZero() || t2.Before(t) {
			t = t2
		}
	}
	return t
}

// Keep returns false if a snapshot at the given time is old and should be deleted.
// Always returns false if this SnapshotWindows is empty.
func (sws SnapshotWindows) Keep(snapshotT time.Time) bool {
	return sws.keep(time.Now(), snapshotT)
}

func (sws SnapshotWindows) keep(now, snapshotT time.Time) bool {
	d := now.Sub(snapshotT)
	for _, sw := range sws {
		if d <= sw.KeepFor && ptime.IsMultiple(snapshotT, sw.Every) {
			return true
		}
	}
	return false
}
