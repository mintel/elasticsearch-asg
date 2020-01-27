package time

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert" // Test assertions e.g. equality
)

func TestRoundedTicker(t *testing.T) {
	d := time.Millisecond
	ticker := NewRoundedTicker(d)
	for i := 0; i < 10; i++ {
		tick := <-ticker.C
		assert.Equal(t, tick, tick.Round(d))
	}
}

func TestRoundedTicker_skip(t *testing.T) {
	now := time.Now()
	d := 100 * time.Millisecond
	ticker := NewRoundedTicker(d)
	time.Sleep(3 * d)
	tick := <-ticker.C
	assert.True(t, tick.After(now.Round(d)))
}

// Copy the Ticker tests from Go stdlib.

func TestRoundedTicker_go(t *testing.T) {
	const Count = 10
	Delta := 100 * time.Millisecond
	ticker := NewRoundedTicker(Delta)
	t0 := time.Now()
	for i := 0; i < Count; i++ {
		<-ticker.C
	}
	ticker.Stop()
	t1 := time.Now()
	dt := t1.Sub(t0)
	target := Delta * Count
	slop := target * 2 / 10
	if dt < target-slop || (!testing.Short() && dt > target+slop) {
		t.Fatalf("%d %s ticks took %s, expected [%s,%s]", Count, Delta, dt, target-slop, target+slop)
	}
	// Now test that the ticker stopped
	time.Sleep(2 * Delta)
	select {
	case <-ticker.C:
		t.Fatal("Ticker did not shut down")
	default:
		// ok
	}
}

// Issue 21874
func TestRoundedTickerStopWithDirectInitialization_go(t *testing.T) {
	c := make(chan time.Time)
	tk := &RoundedTicker{C: c}
	tk.Stop()
}

// Test that a bug tearing down a ticker has been fixed. This routine should not deadlock.
func TestRoundedTickerTeardown_go(t *testing.T) {
	Delta := 100 * time.Millisecond
	if testing.Short() {
		Delta = 20 * time.Millisecond
	}
	for i := 0; i < 3; i++ {
		ticker := NewRoundedTicker(Delta)
		<-ticker.C
		ticker.Stop()
	}
}

// Test that NewTicker panics when given a duration less than zero.
func TestNewRoundedTickerZeroDuration_go(t *testing.T) {
	defer func() {
		if err := recover(); err == nil {
			t.Errorf("NewRoundedTicker(-1) should have panicked")
		}
	}()
	NewRoundedTicker(-1)
}
