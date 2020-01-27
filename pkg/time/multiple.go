package time

import (
	"errors"
	"sync"
	"time"
)

// Ceil returns the result of rounding t up to a multiple of d (since the zero time).
// If d <= 0, Ceil returns t unchanged.
func Ceil(t time.Time, d time.Duration) time.Time {
	if d <= 0 {
		return t
	}
	return t.Add(d).Truncate(d)
}

// Prev returns the nearest multiple of d before t (since the zero time).
// If d <= 0, Prev returns t unchanged.
func Prev(t time.Time, d time.Duration) time.Time {
	if d <= 0 {
		return t
	}
	t2 := t.Truncate(d)
	if t2.Equal(t) {
		t2 = t2.Add(-d)
	}
	return t2
}

// Next returns the nearest multiple of d after t (since the zero time).
// If d <= 0, Next returns t unchanged.
func Next(t time.Time, d time.Duration) time.Time {
	if d <= 0 {
		return t
	}
	return t.Truncate(d).Add(d)
}

// IsMultiple returns true if t is some multiple of d (since the zero time).
// If d <= 0, IsMultiple returns false.
func IsMultiple(t time.Time, d time.Duration) bool {
	if d <= 0 {
		return false
	}
	return t.Truncate(d).Equal(t)
}

// RoundedTicker is like a time.Ticker, but rounded up to
// the nearest multiple of the tick Duration from the zero time.
type RoundedTicker struct {
	C <-chan time.Time

	c        chan<- time.Time
	d        time.Duration
	once     sync.Once
	stopping chan struct{}
}

// NewRoundedTicker returns a new RoundedTicker.
func NewRoundedTicker(d time.Duration) *RoundedTicker {
	if d <= 0 {
		panic(errors.New("non-positive interval for NewRoundedTicker"))
	}
	c := make(chan time.Time)
	rt := &RoundedTicker{
		C:        c,
		c:        c,
		d:        d,
		stopping: make(chan struct{}),
	}
	go rt.run()
	return rt
}

func (rt *RoundedTicker) run() {
	nextTick := Ceil(time.Now(), rt.d)
	doTick := time.NewTimer(time.Until(nextTick))
	defer doTick.Stop()
	for {
		select {
		case <-doTick.C:
			t := nextTick
			nextTick = nextTick.Add(rt.d)
			doTick.Reset(time.Until(nextTick))
			select {
			case rt.c <- t:
				// noop
			default:
				// noop
			}
		case <-rt.stopping:
			return
		}
	}
}

// Stop turns off a ticker. After Stop, no more ticks will be sent.
// Stop does not close the channel, to prevent a concurrent goroutine reading from
// the channel from seeing an erroneous "tick".
func (rt *RoundedTicker) Stop() {
	rt.once.Do(func() {
		// Check if nil just in case RoundedTicker was directly initialized.
		// https://github.com/golang/go/issues/21874
		if rt.stopping != nil {
			close(rt.stopping)
		}
	})
}
