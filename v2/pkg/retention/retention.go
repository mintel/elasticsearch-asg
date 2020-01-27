// Package retention implements a stateless algorithm for determining which
// of a sequence of backups to retain with decreasing granularity over time.
// It is based on the retention algorithm used by Acronis Disaster Recovery Service.
//
// See also: https://kb.acronis.com/content/58486
//
// Since both "backups" and "buckets" start with "b", hereafter the word
// "snapshots" will be used instead of "backups" for readability.
//
// Snapshots using this method should follow a few principals:
// 1. Use Config.MinInterval to schedule the snapshot jobs.
// 2. If a previous snapshot job has not finished yet, the next one doesn't run.
//
// Example Use:
//
// 	c := retention.Config{
// 		Hourly:  24,
// 		Daily:   6,
// 		Weekly:  4,
// 		Monthly: 11,
// 		Yearly:  1,
// 	}
// 	for range time.Tick(c.MinInterval()) {
// 		DoSnapshot()
// 		backupTimes := ListSnapshots()
// 		toDelete := retention.Delete(c, backupTimes)
// 		DeleteSnapshots(toDelete)
// 	}
//
package retention

import (
	"math"
	"time"
)

// Keep takes a list of times that snapshots were taken at and
// returns which ones should be kept.
//
// The retention policy is based on the last snapshot time,
// not use the current time.
// Times will be returned in sorted order.
func Keep(c Config, snapshots []time.Time) []time.Time {
	shots := make(timeseries, 0, len(snapshots)+1)
	shots.Push(snapshots...)
	newest := shots.PeekNewest()
	buks := newBuckets(c, newest)
	buks.Assign(shots)

	// Redistribute the snapshots between buckets with the
	// goal of having 1 snapshot per bucket.
	redistributeBackups(buks)

	// Do an initial pass marking snapshots to keep.
	keep := make(timeseries, 0, len(buks))
	for i, b := range buks {
		// Don't keep any snapshots in fall
		// before our defined buckets.
		if b.IsCatchall() {
			continue
		}
		// Keep all snapshots in hour buckets.
		if b.Width == Hour {
			keep.Push(b.Snapshots...)
			continue
		}
		// Keep the oldest snapshot in each bucket.
		keep.Push(b.Snapshots.PeekOldest())
		// Keep the very newest snapshot of all.
		if i == len(buks)-1 {
			keep.Push(b.Snapshots.PeekNewest())
		}
	}

	// Per Acronis:
	//
	//   Go through each interval spanning 2 snapshots marked to keep
	//   (with at least 1.5 bucket length between them)
	//   and find one snapshot marked for deletion in that interval
	//   which is the closest to the middle of the interval
	//   (but no closer than 0.5 of the bucket length from either
	//   beginning or ending of the interval).
	//   If such a snapshot is found then it is marked for retention
	//   and won't be deleted.
	keep2 := make(timeseries, 0, len(keep)/2)
	for i, n := 0, len(keep)-1; i < n; i++ {
		snapLeft, snapRight := keep[i], keep[i+1]
		bucketLeft, bucketRight := buks.Has(snapLeft), buks.Has(snapRight)
		if bucketLeft == bucketRight {
			// The same bucket can't be 1.5 bucket length apart.
			continue
		}

		// The Acronis algorithm doesn't really define what "1.5 bucket length"
		// is when the buckets are different sizes.
		// I think the geometric mean is the correct way to go.
		bucketLength := math.Sqrt(float64(bucketLeft.Width) * float64(bucketRight.Width))
		distance := snapRight.Sub(snapLeft)
		if float64(distance) < 1.5*bucketLength {
			continue
		}

		center := snapLeft.Add(distance / 2)
		end := shots.Find(snapRight)
		closestDistance := distance
		var closestSnap time.Time
		for i := shots.Find(snapLeft) + 1; i < end; i++ {
			snapMiddle := shots[i]
			distanceLeft := float64(snapMiddle.Sub(snapLeft))
			if distanceLeft < 0.5*bucketLength {
				continue
			}
			distanceRight := float64(snapRight.Sub(snapMiddle))
			if distanceRight < 0.5*bucketLength {
				continue
			}
			d := center.Sub(snapMiddle)
			if d < 0 {
				d = -d
			}
			if d < closestDistance {
				closestDistance = d
				closestSnap = snapMiddle
			}
		}
		if !closestSnap.IsZero() {
			keep2.Push(closestSnap)
		}
	}

	keep.Push(keep2...)
	return keep
}

// Delete is the inverse of Keep, returning snapshots to delete.
func Delete(c Config, snapshots []time.Time) []time.Time {
	keep := Keep(c, snapshots)
	shots := make(timeseries, 0, len(snapshots))
	shots.Push(snapshots...)
	shots.Discard(keep...)
	return []time.Time(shots)
}

// redistributeBackups moves snapshots between buckets
// to more evenly distribute them. The ideal (but unlikely)
// result of this algorithm is that each bucket will
// have 1 backup.
func redistributeBackups(buckets buckets) {
	// close is 1/4 the width of the smallest bucket.
	close := buckets[len(buckets)-1].Width / 4

	for i := 0; i < len(buckets)-1; i++ {
		b, nb := buckets[i], buckets[i+1]

		if len(b.Snapshots) == 0 {
			// If this bucket has no snapshots and next newest
			// bucket has a backup that is close to the boundary
			// between, reassign that backup to this bucket.
			if len(nb.Snapshots) == 0 {
				continue
			}
			distance := nb.Snapshots.PeekOldest().Sub(b.End)
			if distance <= close {
				snap := nb.Snapshots.PopOldest()
				b.Snapshots.Push(snap)
			}

		} else if len(b.Snapshots) > 1 {
			// If this bucket has more than one backup and one
			// of the snapshots is close to the boundary with the
			// next newest bucket, reassign that backup to the
			// newer bucket.
			distance := b.End.Sub(b.Snapshots.PeekNewest())
			if distance <= close {
				snap := b.Snapshots.PopNewest()
				nb.Snapshots.Push(snap)
			}
		}
	}
}
