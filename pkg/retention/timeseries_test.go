package retention

import (
	"math/rand"
	"reflect"
	"sort"
	"testing"
	"testing/quick"
	"time"

	"github.com/stretchr/testify/assert" // Test assertions e.g. equality.
)

func Test_timeseries_Push(t *testing.T) {
	t.Run("canonical-shuffle", func(t *testing.T) {
		_, want, _, _, _, _ := canonical()
		hasProperties := func(timesets [][]time.Time, cap int) bool {
			ts := make(timeseries, 0, cap)
			for _, times := range timesets {
				ts.Push(times...)
			}
			return assertTimeseriesSorted(t, ts) && assert.Exactly(t, timeseries(want), ts)
		}
		testValues := func(v []reflect.Value, r *rand.Rand) {
			times := append([]time.Time(nil), want...)
			r.Shuffle(len(times), func(i, j int) {
				times[i], times[j] = times[j], times[i]
			})
			var timesets [][]time.Time
			i := 0
			n := r.Intn(len(times)) + 1
			for i < len(times) {
				timesets = append(timesets, times[i:i+n])
				i += n
				if i < len(times) {
					n = r.Intn(len(times)-i) + 1
				}
			}
			v[0] = reflect.ValueOf(timesets)
			v[1] = reflect.ValueOf(r.Intn(len(times) + 5))
		}
		c := &quick.Config{Values: testValues}
		if err := quick.Check(hasProperties, c); err != nil {
			t.Error(err)
		}
	})

	t.Run("quick-random", func(t *testing.T) {
		hasProperties := func(times1, times2 []time.Time) bool {
			ts := make(timeseries, 0, len(times1))
			ts.Push(times1...)
			pass := assertTimeseriesSorted(t, ts)
			pass = pass && assert.Subset(t, ts, times1)
			ts.Push(times2...)
			pass = pass && assertTimeseriesSorted(t, ts)
			pass = pass && assert.Subset(t, ts, times2)

			return pass
		}
		testValues := func(v []reflect.Value, r *rand.Rand) {
			var times1 []time.Time
			n := r.Intn(50) + 2
			for i := 0; i < n; i++ {
				t := time.Unix(r.Int63(), r.Int63())
				times1 = append(times1, t)
			}
			sort.Slice(times1, func(i, j int) bool {
				return times1[i].Before(times1[j])
			})
			v[0] = reflect.ValueOf(times1)

			// Pushing to an empty timeseries is trivial.
			// Also try pushing times before, in the middle,
			// non-unique, and after.
			times2 := []time.Time{
				times1[0].Add(-time.Second),
				times1[0],
				times1[0].Add(times1[1].Sub(times1[0])),
				times1[len(times1)-1].Add(time.Second),
			}
			v[1] = reflect.ValueOf(times2)
		}
		c := &quick.Config{Values: testValues}
		if err := quick.Check(hasProperties, c); err != nil {
			t.Error(err)
		}
	})
}

func Test_timeseries_Pop(t *testing.T) {
	hasProperties := func(ts timeseries, i int) bool {
		if i >= len(ts) {
			return assert.Panics(
				t,
				func() {
					ts.Pop(i)
				},
				"Pop of non-existance index didn't panic",
			)
		}
		n := len(ts)
		v := ts.Pop(i)
		pass := assertTimeseriesSorted(t, ts)
		pass = pass && assert.Len(
			t, ts, n-1,
			"popped timeseries has wrong length",
		)
		if i < n-1 {
			pass = pass && assert.True(
				t, v.Before(ts[i]),
				"new time at same index isn't newer",
			)
		}
		if i > 0 {
			pass = pass && assert.True(
				t, v.After(ts[i-i]),
				"time before popped index isn't older",
			)
		}
		return pass
	}
	testValues := func(v []reflect.Value, r *rand.Rand) {
		ts := randomTimeseries(r, r.Intn(50))
		v[0] = reflect.ValueOf(ts)
		v[1] = reflect.ValueOf(r.Intn(len(ts) + 1))
	}
	c := &quick.Config{Values: testValues}
	if err := quick.Check(hasProperties, c); err != nil {
		t.Error(err)
	}
}

func Test_timeseries_Discard(t *testing.T) {
	hasProperties := func(ts timeseries, toDiscard []time.Time) bool {
		n := len(ts)
		ts.Discard(toDiscard...)
		pass := assertTimeseriesSorted(t, ts)
		pass = pass && assert.Len(
			t, ts, n-(len(toDiscard)-1),
			"length of timeseries after Discard is wrong",
		)
		for _, time := range toDiscard {
			pass = pass && assert.Equal(
				t, -1, ts.Find(time),
				"Discarded time still in timeseries",
			)
		}
		return pass
	}
	testValues := func(v []reflect.Value, r *rand.Rand) {
		var ts timeseries
		for len(ts) == 0 {
			ts = randomTimeseries(r, r.Intn(50))
		}
		v[0] = reflect.ValueOf(ts)
		toDiscard := []time.Time{
			ts[0].Add(-time.Second), // A time that isn't in the series shouldn't cause a problem.
		}
		for i := 0; i < len(ts); i += r.Intn(len(ts)-i) + 1 {
			toDiscard = append(toDiscard, ts[i])
		}
		v[1] = reflect.ValueOf(toDiscard)
	}
	c := &quick.Config{Values: testValues}
	if err := quick.Check(hasProperties, c); err != nil {
		t.Error(err)
	}
}

func Test_timeseries_PopOldest(t *testing.T) {
	hasProperties := func(ts timeseries) bool {
		n := len(ts)
		v := ts.PopOldest()
		if n == 0 {
			return v.IsZero()
		}
		pass := assertTimeseriesSorted(t, ts)
		pass = pass && assert.Len(
			t, ts, n-1,
			"popped timeseries has wrong length",
		)
		for i := 0; i < len(ts); i++ {
			pass = pass && assert.True(
				t, v.Before(ts[i]),
				"value in timeseries older than popped oldest: %s (index %d) < %s",
				ts[i], i, v,
			)
		}
		return pass
	}
	testValues := func(v []reflect.Value, r *rand.Rand) {
		v[0] = reflect.ValueOf(randomTimeseries(r, r.Intn(50)))
	}
	c := &quick.Config{Values: testValues}
	if err := quick.Check(hasProperties, c); err != nil {
		t.Error(err)
	}
}

func Test_timeseries_PeakOldest(t *testing.T) {
	hasProperties := func(ts timeseries) bool {
		n := len(ts)
		v := ts.PeakOldest()
		if n == 0 {
			return v.IsZero()
		}
		pass := assertTimeseriesSorted(t, ts)
		pass = pass && assert.Len(
			t, ts, n,
			"peaked timeseries has different length",
		)
		for i := 0; i < len(ts); i++ {
			if i == 0 {
				pass = pass && assert.True(
					t, v.Equal(ts[i]),
					"peaked oldest not equal to oldest in timeseries: %s < %s",
					ts[i], v,
				)
			} else {
				pass = pass && assert.True(
					t, v.Before(ts[i]),
					"peakold oldest not oldest in timeseries: %s (index %d) < %s",
					ts[i], i, v,
				)
			}
		}
		return pass
	}
	testValues := func(v []reflect.Value, r *rand.Rand) {
		v[0] = reflect.ValueOf(randomTimeseries(r, r.Intn(50)))
	}
	c := &quick.Config{Values: testValues}
	if err := quick.Check(hasProperties, c); err != nil {
		t.Error(err)
	}
}

func Test_timeseries_PopNewest(t *testing.T) {
	hasProperties := func(ts timeseries) bool {
		n := len(ts)
		v := ts.PopNewest()
		if n == 0 {
			return v.IsZero()
		}
		pass := assertTimeseriesSorted(t, ts)
		pass = pass && assert.Len(
			t, ts, n-1,
			"popped timeseries has wrong length",
		)
		for i := 0; i < len(ts); i++ {
			pass = pass && assert.True(
				t, v.After(ts[i]),
				"popped newest isn't newer than value in timeseries: %s (index %d) > %s",
				ts[i], i, v,
			)
		}
		return pass
	}
	testValues := func(v []reflect.Value, r *rand.Rand) {
		v[0] = reflect.ValueOf(randomTimeseries(r, r.Intn(50)))
	}
	c := &quick.Config{Values: testValues}
	if err := quick.Check(hasProperties, c); err != nil {
		t.Error(err)
	}
}

func Test_timeseries_PeakNewest(t *testing.T) {
	hasProperties := func(ts timeseries) bool {
		n := len(ts)
		v := ts.PeakNewest()
		if n == 0 {
			return v.IsZero()
		}
		pass := assertTimeseriesSorted(t, ts)
		pass = pass && assert.Len(
			t, ts, n,
			"peaked timeseries has a different length",
		)
		for i := 0; i < len(ts); i++ {
			if i < len(ts)-1 {
				pass = pass && assert.True(
					t, ts[i].Before(ts[i+1]),
					"peaked newest isn't newer than timeseries newest: %s (index %d) > %s",
					ts[i], i, v,
				)
			} else {
				pass = pass && assert.True(
					t, v.Equal(ts[i]),
					"peaked newest isn't newer than other value: %s (index %d) > %s",
					ts[i], i, v,
				)
			}
		}
		return pass
	}
	testValues := func(v []reflect.Value, r *rand.Rand) {
		v[0] = reflect.ValueOf(randomTimeseries(r, r.Intn(50)))
	}
	c := &quick.Config{Values: testValues}
	if err := quick.Check(hasProperties, c); err != nil {
		t.Error(err)
	}
}

func Test_timeseries_Find(t *testing.T) {
	hasProperties := func(ts timeseries, time time.Time, want int) bool {
		got := ts.Find(time)
		return assert.Equal(t, want, got)
	}
	testValues := func(v []reflect.Value, r *rand.Rand) {
		ts := randomTimeseries(r, r.Intn(50))
		v[0] = reflect.ValueOf(ts)
		if len(ts) == 0 {
			// Empty timeseries.Find should always return -1.
			v[1] = reflect.ValueOf(time.Unix(r.Int63(), r.Int63()))
			v[2] = reflect.ValueOf(-1)
			return
		}
		if r.Float64() > 0.5 {
			i := r.Intn(len(ts))
			v[1] = reflect.ValueOf(ts[i])
			v[2] = reflect.ValueOf(i)
		} else {
			// Times not in timeseries should return -1.
			v[1] = reflect.ValueOf(ts[0].Add(-time.Second))
			v[2] = reflect.ValueOf(-1)
		}
	}
	c := &quick.Config{Values: testValues}
	if err := quick.Check(hasProperties, c); err != nil {
		t.Error(err)
	}
}

// randomTimeseries returns a random timeseries of length n.
func randomTimeseries(r *rand.Rand, n int) timeseries {
	ts := make(timeseries, 0, n)
	for len(ts) < n {
		ts.Push(time.Unix(r.Int63(), r.Int63()))
	}
	return ts
}

// randomTimeseriesBetween returns a random timeseries of length n
// between start and end times.
func randomTimeseriesBetween(r *rand.Rand, n int, start, end time.Time) timeseries {
	if end.Before(start) {
		end, start = start, end
	}
	diff := end.Sub(start) + 1
	ts := make(timeseries, 0, n)
	for len(ts) < n {
		d := time.Duration(float64(diff) * r.Float64())
		ts.Push(start.Add(d))
	}
	return ts
}

// assertTimeseriesSorted asserts that a timeseries is in sorted order.
func assertTimeseriesSorted(t *testing.T, ts timeseries) bool {
	pass := true
	for i := 0; i < len(ts)-1; i++ {
		pass = pass && assert.True(
			t, ts[i].Before(ts[i+1]),
			"timeseries isn't sorted: %s (index %d) -> %s (index %d)",
			ts[i], i, ts[i+1], i+1,
		)
	}
	return pass
}

func Benchmark_timeseries_Push(b *testing.B) {
	var ts timeseries
	noise := make([]byte, b.N)
	if _, err := rand.Read(noise); err != nil {
		b.Error(err)
	}
	b.ResetTimer()
	for _, n := range noise {
		d := time.Duration(n)
		if d%2 == 0 {
			d = -d
		}
		t := time.Now().Add(d)
		ts.Push(t)
	}
}

func Benchmark_timeseries_Push_many(b *testing.B) {
	var ts timeseries
	noise := make([]byte, b.N)
	if _, err := rand.Read(noise); err != nil {
		b.Error(err)
	}
	times := make([]time.Time, b.N)
	for i, n := range noise {
		d := time.Duration(n)
		if d%2 == 0 {
			d = -d
		}
		t := time.Now().Add(d)
		times[i] = t
	}
	b.ResetTimer()
	ts.Push(times...)
}
