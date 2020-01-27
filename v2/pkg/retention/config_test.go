package retention

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert" // Test assertions e.g. equality.
)

func TestConfig_MinInterval(t *testing.T) {
	hasProperties := func(c Config) bool {
		got := c.MinInterval()
		pass := true
		switch got {
		case -1:
			pass = pass && assert.Zero(t, c.Hourly, "Hourly wasn't zero")
			pass = pass && assert.Zero(t, c.Daily, "Daily wasn't zero")
			pass = pass && assert.Zero(t, c.Weekly, "Weekly wasn't zero")
			pass = pass && assert.Zero(t, c.Monthly, "Monthly wasn't zero")
			pass = pass && assert.Zero(t, c.Yearly, "Yearly wasn't zero")
		case Hour:
			pass = pass && assert.NotZero(t, c.Hourly, "Hourly is zero")
		case Day:
			pass = pass && assert.Zero(t, c.Hourly, "Hourly is zero")
			pass = pass && assert.NotZero(t, c.Daily, "Daily is zero")
		case Week:
			pass = pass && assert.Zero(t, c.Hourly, "Hourly is zero")
			pass = pass && assert.Zero(t, c.Daily, "Daily is zero")
			pass = pass && assert.NotZero(t, c.Weekly, "Weekly is zero")
		case Month:
			pass = pass && assert.Zero(t, c.Hourly, "Hourly is zero")
			pass = pass && assert.Zero(t, c.Daily, "Daily is zero")
			pass = pass && assert.Zero(t, c.Weekly, "Weekly is zero")
			pass = pass && assert.NotZero(t, c.Monthly, "Monthly is zero")
		case Year:
			pass = pass && assert.Zero(t, c.Hourly, "Hourly is zero")
			pass = pass && assert.Zero(t, c.Daily, "Daily is zero")
			pass = pass && assert.Zero(t, c.Weekly, "Weekly is zero")
			pass = pass && assert.Zero(t, c.Monthly, "Monthly is zero")
			pass = pass && assert.NotZero(t, c.Yearly, "Yearly is zero")
		default:
			return assert.Fail(t, "got unknown min interval")
		}
		return pass
	}
	testValues := func(v []reflect.Value, r *rand.Rand) {
		v[0] = reflect.ValueOf(randomConfig(r, true, 0))
	}
	c := &quick.Config{Values: testValues}
	if err := quick.Check(hasProperties, c); err != nil {
		t.Error(err)
	}
}

// randomConfig returns a random Config.
func randomConfig(r *rand.Rand, allowZero bool, limit int) Config {
	var f func() uint
	if limit > 0 {
		f = func() uint { return uint(r.Intn(limit)) }
	} else {
		f = func() uint { return uint(r.Int()) }
	}
	c := Config{
		Hourly:  f(),
		Daily:   f(),
		Weekly:  f(),
		Monthly: f(),
		Yearly:  f(),
	}
	for !allowZero && c.len() == 0 {
		c.Hourly = f()
		c.Daily = f()
		c.Weekly = f()
		c.Monthly = f()
		c.Yearly = f()
	}
	return c
}
