package time

import (
	"errors"
	"sync"
	"time"
)

// RoundedTicker is like a time.Ticker, but rounded up to
// the nearest multiple of the tick Duration from the Unix
// Epoch.
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
	nextTick := time.Now().Truncate(rt.d).Add(rt.d)
	startTick := time.NewTimer(time.Until(nextTick))
	c := rt.c
	for {
		select {
		case <-rt.stopping:
			if !startTick.Stop() {
				<-startTick.C
			}
			return
		case <-startTick.C:
			// Send tick non-blocking
			go func(t time.Time) {
				select {
				case c <- t:
					// noop
				default:
					// noop
				}
			}(nextTick)
			nextTick = nextTick.Add(rt.d)
			startTick.Reset(time.Until(nextTick))
		}
	}
}

// Stop turns off a ticker. After Stop, no more ticks will be sent.
// Stop does not close the channel, to prevent a concurrent goroutine reading from
// the channel from seeing an erroneous "tick".
func (rt *RoundedTicker) Stop() {
	rt.once.Do(func() {
		if rt.stopping != nil {
			close(rt.stopping)
		}
	})
}
