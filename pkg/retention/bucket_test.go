package retention

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_bucket_Start(t *testing.T) {
	t.Run("quick", func(t *testing.T) {
		hasProperties := func(w time.Duration, e time.Time) bool {
			b := &bucket{
				Width: w,
				End:   e,
			}
			got := b.Start()
			d := e.Sub(got)
			pass := assert.True(t, got.Before(e), "bucket start not before bucket end")
			pass = pass && assert.Equal(t, w, d, "bucket end - width != bucket start")
			return pass
		}
		testValues := func(v []reflect.Value, r *rand.Rand) {
			var w time.Duration
			for w == 0 {
				w = time.Duration(r.Int63())
			}
			v[0] = reflect.ValueOf(w)
			v[1] = reflect.ValueOf(time.Unix(r.Int63(), r.Int63()))
		}
		c := &quick.Config{Values: testValues}
		if err := quick.Check(hasProperties, c); err != nil {
			t.Error(err)
		}
	})
}

func Test_newBuckets(t *testing.T) {
	t.Run("canonical", func(t *testing.T) {
		c, times, want, _, _, _ := canonical()
		got := newBuckets(c, times[len(times)-1])
		if !assert.Equal(t, len(want), len(got)) {
			return
		}
		for i := range got {
			assert.WithinDuration(
				t, want[i].End, got[i].End, 0,
				"bucket %d end doesn't match", i,
			)
			assert.Equal(
				t, want[i].Width, got[i].Width,
				"bucket %d width doesn't match", i,
			)
		}
	})

	t.Run("quick", func(t *testing.T) {
		hasProperties := func(c Config, x time.Time) bool {
			got := newBuckets(c, x)

			n := c.len()
			pass := assert.Len(t, got, n+1, "wrong number of buckets")
			if n == 0 {
				return pass
			}

			pass = pass && assert.True(
				t,
				got[len(got)-1].End.Equal(x),
				"End of the last bucket doesn't match passed time.",
			)

			for i := 0; i < n-1; i++ {
				b1, b2 := got[i], got[i+1]

				if i == 0 {
					pass = pass && assert.EqualValues(
						t,
						0,
						b1.Width,
						"Catchall bucket doesn't have Width 0.",
					)
				} else {
					pass = pass && assert.True(
						t,
						b1.Start().Before(b2.Start()),
						"Left bucket doesn't start before right bucket.",
					)

					pass = pass && assert.GreaterOrEqual(
						t,
						int64(b1.Width),
						int64(b2.Width),
						"Buckets aren't in the right order (%s => %s).",
						b1.Width, b2.Width,
					)
				}

				pass = pass && assert.True(
					t,
					b1.End.Equal(b2.Start()),
					"Bucket boundaries don't align.",
				)
			}

			return pass
		}
		testValues := func(v []reflect.Value, r *rand.Rand) {
			v[0] = reflect.ValueOf(randomConfig(r, true, 10))
			v[1] = reflect.ValueOf(time.Unix(r.Int63(), r.Int63()))
		}
		c := &quick.Config{Values: testValues}
		if err := quick.Check(hasProperties, c); err != nil {
			cerr := err.(*quick.CheckError)
			t.Errorf("failed on inputs %+v, %s", cerr.In[0], cerr.In[1])
		}
	})
}

func Test_buckets_For(t *testing.T) {
	t.Run("canonical", func(t *testing.T) {
		c, times, want, _, _, _ := canonical()
		buckets := newBuckets(c, times[len(times)-1])
		buckets.Assign(times)
		for i := range want {
			for j, s := range want[i].Snapshots {
				got := buckets.For(s)
				assert.Equal(
					t, want[i], got,
					"bucket %d does match for time %d", i, j,
				)
			}
		}
	})

	t.Run("quick", func(t *testing.T) {
		hasProperties := func(c Config, end time.Time) bool {
			buckets := newBuckets(c, end)
			start := buckets[0].Start()

			d := end.Sub(start)
			offset := time.Duration(rand.Int63n(int64(d)))
			middle := start.Add(offset)
			got := buckets.For(middle)
			if !assert.NotNil(t, got) {
				return false
			}
			pass := assert.True(
				t,
				(got.Start().Before(middle) || got.Start().Equal(middle)) &&
					(got.End.After(middle) || got.End.Equal(middle)),
				"got wrong bucket: %s is not between %s and %s",
				middle, got.Start(), got.End,
			)

			before := start.Add(-time.Minute)
			got = buckets.For(before)
			if !assert.NotNil(t, got) {
				return false
			}
			pass = pass && assert.EqualValues(
				t, 0, got.Width,
				"didn't get catchall bucket for time before first bucket",
			)
			pass = pass && assert.True(
				t, got.IsCatchall(),
				"didn't get catchall bucket for time before first bucket",
			)

			after := end.Add(time.Minute)
			got = buckets.For(after)
			pass = pass && assert.Nil(
				t, got,
				"didn't get nil for time after last bucket",
			)

			return pass
		}
		testValues := func(v []reflect.Value, r *rand.Rand) {
			v[0] = reflect.ValueOf(randomConfig(r, false, 10))
			v[1] = reflect.ValueOf(time.Unix(r.Int63(), r.Int63()))
		}
		c := &quick.Config{Values: testValues}
		if err := quick.Check(hasProperties, c); err != nil {
			cerr := err.(*quick.CheckError)
			t.Errorf("failed on inputs %+v, %s", cerr.In[0], cerr.In[1])
		}
	})
}

func Test_buckets_Has(t *testing.T) {
	t.Run("canonical", func(t *testing.T) {
		_, _, _, buckets, _, _ := canonical()
		for i, b := range buckets {
			for j, s := range b.Snapshots {
				got := buckets.Has(s)
				assert.Equal(
					t, b, got,
					"bucket %d does match for time %d", i, j,
				)
			}
		}
	})

	t.Run("quick", func(t *testing.T) {
		hasProperties := func(ts timeseries, bs buckets) bool {
			bs.Assign(ts)
			pass := true
			n := 0
			for _, b := range bs {
				pass = pass && assertTimeseriesSorted(t, b.Snapshots)
				n += len(b.Snapshots)
			}
			pass = pass && assert.Equal(t, len(ts), n, "missing snapshots")
			for _, time := range ts {
				got := bs.Has(time)
				pass = pass && assert.Contains(
					t, got.Snapshots, time,
					"snapshot not in expected bucket",
				)
			}
			return pass
		}
		testValues := func(v []reflect.Value, r *rand.Rand) {
			c := randomConfig(r, false, 10)
			bs := newBuckets(c, time.Unix(r.Int63(), r.Int63()))
			ts := randomTimeseriesBetween(
				r,
				r.Intn(50),
				bs.Start(),
				bs.End(),
			)
			v[0] = reflect.ValueOf(ts)
			v[1] = reflect.ValueOf(bs)
		}
		c := &quick.Config{Values: testValues}
		if err := quick.Check(hasProperties, c); err != nil {
			t.Error(err)
		}
	})
}

func Test_buckets_Assign(t *testing.T) {
	t.Run("canonical", func(t *testing.T) {
		c, times, want, _, _, _ := canonical()
		bs := newBuckets(c, times[len(times)-1])
		bs.Assign(times)
		assert.Equal(t, want, bs)
	})

	t.Run("quick", func(t *testing.T) {
		hasProperties := func(ts timeseries, bs buckets) bool {
			bs.Assign(ts)
			pass := true
			n := 0
			for _, b := range bs {
				n += len(b.Snapshots)
				for _, time := range b.Snapshots {
					pass = pass && assert.True(t, b.Start().Equal(time) || b.Start().Before(time), "time before bucket start")
					pass = pass && assert.True(t, b.End.Equal(time) || b.End.After(time), "time after bucket end")
				}
			}
			pass = pass && assert.Equal(t, len(ts), n, "wrong number of snapshots")
			return pass
		}
		testValues := func(v []reflect.Value, r *rand.Rand) {
			c := randomConfig(r, false, 10)
			bs := newBuckets(c, time.Unix(r.Int63(), r.Int63()))
			ts := randomTimeseriesBetween(
				r,
				r.Intn(50),
				bs.Start(),
				bs.End(),
			)
			v[0] = reflect.ValueOf(ts)
			v[1] = reflect.ValueOf(bs)
		}
		c := &quick.Config{Values: testValues}
		if err := quick.Check(hasProperties, c); err != nil {
			t.Error(err)
		}
	})
}
