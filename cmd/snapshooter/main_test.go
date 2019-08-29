package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSnapshotFormat(t *testing.T) {
	want := time.Date(2019, time.May, 5, 5, 26, 13, 0, time.UTC)
	got, err := time.Parse(SnapshotFormat, "snapshooter-2019-05-05t05-26-13")
	if assert.NoError(t, err) {
		assert.WithinDuration(t, want, got, 0)
	}
}
