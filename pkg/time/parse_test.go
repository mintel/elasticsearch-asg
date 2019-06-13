package time

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseISO8601D(t *testing.T) {
	testCases := []struct {
		desc string
		in   string
		want time.Duration
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
			in:   "P10Y5M3DT14H12M8S",
			want: 10*Year + 5*Month + 3*Day + 14*time.Hour + 12*time.Minute + 8*time.Second,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			got, err := ParseISO8601D(tC.in)
			if assert.NoError(t, err) {
				assert.Equal(t, tC.want, got)
			}
		})
	}
}
