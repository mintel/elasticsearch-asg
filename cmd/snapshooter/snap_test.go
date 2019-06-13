package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSnapshotWindows_Next(t *testing.T) {
	tests := []struct {
		name    string
		windows SnapshotWindows
		now     time.Time
		want    time.Time
	}{
		{
			name: "15min",
			windows: windowsFromStrings(
				"PT1H", "PT15M", // snapshots every 15 minutes for 1 hour
				"P1M", "PT1H", // hourly snapshots for 1 month
			),
			now:  time.Date(2019, time.May, 5, 5, 26, 13, 0, time.UTC),
			want: time.Date(2019, time.May, 5, 5, 30, 0, 0, time.UTC),
		},
		{
			name: "hourly",
			windows: windowsFromStrings(
				"PT1H", "PT15M", // snapshots every 15 minutes for 1 hour
				"P1M", "PT1H", // hourly snapshots for 1 month
			),
			now:  time.Date(2019, time.May, 5, 5, 45, 1, 0, time.UTC),
			want: time.Date(2019, time.May, 5, 6, 0, 0, 0, time.UTC),
		},
		{
			name: "weekly",
			windows: windowsFromStrings(
				"P3M", "P1W", // weekly snapshots for 3 months
			),
			now:  time.Date(2019, time.May, 21, 2, 3, 4, 5, time.UTC),
			want: time.Date(2019, time.May, 27, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "monthly",
			windows: windowsFromStrings(
				"P3M", "P1W", // weekly snapshots for 3 months
				"P3Y", "P1M", // monthly snapshots for 3 years
			),
			now:  time.Date(2019, time.June, 1, 0, 0, 0, 0, time.UTC),
			want: time.Date(2019, time.June, 2, 13, 11, 6, 0, time.UTC), // date math... (sigh)
		},
	}

	for _, tC := range tests {
		t.Run(tC.name, func(t *testing.T) {
			got := tC.windows.next(tC.now)
			assert.WithinDuration(t, tC.want, got, 0)
		})
	}
}

func TestSnapshotWindows_Keep(t *testing.T) {
	windows := windowsFromStrings(
		"P1M", "PT1H", // hourly snapshots for one month
		"P3M", "P1W", // weekly snapshots for one quarter
		"P3Y", "P1M", // monthly snapshots for 3 years
	)

	tests := []struct {
		name  string
		desc  string
		taken time.Time
		now   time.Time
		want  bool
	}{
		{
			name:  "hourly_start",
			desc:  "hourly snapshot should be kept",
			taken: time.Date(2019, time.May, 5, 5, 0, 0, 0, time.UTC),
			now:   time.Date(2019, time.May, 5, 5, 26, 13, 0, time.UTC),
			want:  true,
		},
		{
			name:  "hourly_end",
			desc:  "hourly snapshot from almost a month ago should be kept",
			taken: time.Date(2019, time.April, 5, 6, 0, 0, 0, time.UTC),
			now:   time.Date(2019, time.May, 5, 5, 26, 13, 0, time.UTC),
			want:  true,
		},
		{
			name:  "hourly_after",
			desc:  "hourly snapshot from over a month ago should not be kept",
			taken: time.Date(2019, time.April, 5, 5, 0, 0, 0, time.UTC),
			now:   time.Date(2019, time.May, 5, 15, 29, 6, 1, time.UTC),
			want:  false,
		},
		{
			name:  "weekly_start",
			desc:  "weekly snapshot from over a month ago should be kept",
			taken: time.Date(2019, time.April, 1, 0, 0, 0, 0, time.UTC),
			now:   time.Date(2019, time.May, 5, 5, 26, 13, 0, time.UTC),
			want:  true,
		},
		{
			name:  "weekly_end",
			desc:  "weekly snapshot from almost a 3 months ago should be kept",
			taken: time.Date(2019, time.February, 4, 0, 0, 0, 0, time.UTC),
			now:   time.Date(2019, time.May, 5, 5, 26, 13, 0, time.UTC),
			want:  true,
		},
		{
			name:  "weekly_after",
			desc:  "weekly snapshot from over 3 months ago should be not kept",
			taken: time.Date(2019, time.January, 28, 0, 0, 0, 0, time.UTC),
			now:   time.Date(2019, time.May, 5, 5, 26, 13, 0, time.UTC),
			want:  false,
		},
		// Months are treated as 30.436875 days, so these won't align nicely with T00:00:00.000000.
		{
			name:  "monthly_start",
			desc:  "monthy snapshot from over 3 months ago should be kept",
			taken: time.Date(2019, time.January, 31, 19, 14, 42, 0, time.UTC),
			now:   time.Date(2019, time.May, 5, 5, 26, 13, 0, time.UTC),
			want:  true,
		},
		{
			name:  "monthly_end",
			desc:  "monthy snapshot from almost 3 years ago should be kept",
			taken: time.Date(2016, time.June, 1, 19, 43, 30, 0, time.UTC),
			now:   time.Date(2019, time.May, 5, 5, 26, 13, 0, time.UTC),
			want:  true,
		},
		{
			name:  "monthly_after",
			desc:  "monthy snapshot from over 3 years ago should be not kept",
			taken: time.Date(2016, time.May, 2, 9, 14, 24, 0, time.UTC),
			now:   time.Date(2019, time.May, 5, 5, 26, 13, 0, time.UTC),
			want:  false,
		},
	}

	for _, tC := range tests {
		t.Run(tC.name, func(t *testing.T) {
			got := windows.keep(tC.now, tC.taken)
			assert.Equal(t, tC.want, got, tC.desc)
		})
	}
}

func TestSnapshotFormat(t *testing.T) {
	want := time.Date(2019, time.May, 5, 5, 26, 13, 0, time.UTC)
	got, err := time.Parse(SnapshotFormat, "2019-05-05-05-26-13")
	if assert.NoError(t, err) {
		assert.WithinDuration(t, want, got, 0)
	}
}

func windowsFromStrings(s ...string) SnapshotWindows {
	var windows SnapshotWindows
	for i := 0; i < len(s); i += 2 {
		w, err := NewSnapshotWindow(s[i+1], s[i])
		if err != nil {
			panic(err)
		}
		windows = append(windows, w)
	}
	return windows
}
