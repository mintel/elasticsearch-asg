package drainer

import (
	"strings"
	"time"

	"github.com/olebedev/emitter"
)

// topicKey joins strings to make an event topic key.
func topicKey(parts ...string) string {
	return strings.Join(parts, ":")
}

// batchEvents collects events that occur close together and sends
// them in a batch.
func batchEvents(in <-chan emitter.Event, out chan []emitter.Event, timeout time.Duration, max int) <-chan []emitter.Event {
	go func() {
		for {
			b := make([]emitter.Event, 1)
			e, ok := <-in
			if !ok {
				close(out)
				return
			}
			b[0] = e
			t := time.NewTimer(timeout)
		loop:
			for len(b) < max {
				select {
				case e := <-in:
					b = append(b, e)
					t.Reset(timeout)
				case <-t.C:
					break loop
				}
			}
			t.Stop()
			out <- b
		}
	}()
	return out
}
