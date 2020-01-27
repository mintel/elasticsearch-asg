package time

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert" // Test assertions e.g. equality
)

func TestParseISO8601D(t *testing.T) {
	testCases := []struct {
		desc string
		in   string
		want time.Duration
		err  error
	}{
		{
			desc: "zero",
			in:   "P0",
			want: 0,
		},
		{
			desc: "week",
			in:   "P1W",
			want: 7 * Day,
		},
		{
			desc: "year",
			in:   "P1Y",
			want: Year,
		},
		{
			desc: "month",
			in:   "P1M",
			want: Month,
		},
		{
			desc: "day",
			in:   "P1D",
			want: Day,
		},
		{
			desc: "hour",
			in:   "PT1H",
			want: time.Hour,
		},
		{
			desc: "minute",
			in:   "PT1M",
			want: time.Minute,
		},
		{
			desc: "second",
			in:   "PT1S",
			want: time.Second,
		},
		{
			desc: "complex",
			in:   "P10Y5M-3DT14H12M8S",
			want: 10*Year + 5*Month - 3*Day + 14*time.Hour + 12*time.Minute + 8*time.Second,
		},
		{
			desc: "overflow-positive",
			in:   "P292Y3M9DT20H53M35S",
			want: time.Duration(math.MaxInt64),
			err:  ErrInt64Overflow,
		},
		{
			desc: "overflow-negative",
			in:   "P-292Y-3M-9DT-20H-53M-35S",
			want: time.Duration(math.MinInt64),
			err:  ErrInt64Overflow,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			got, err := ParseISO8601D(tC.in)
			assert.Equal(t, tC.err, err)
			assert.Equal(t, tC.want, got)
		})
	}
}
