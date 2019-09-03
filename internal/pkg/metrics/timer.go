package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// VecTimer is a helper type to time functions.
// It is similar to prometheus.Timer, but takes a prometheus.ObserverVec,
// and can add labels to it when the VecTimer is observed.
// Use NewVecTimer to create new instances.
type VecTimer struct {
	begin time.Time
	vec   prometheus.ObserverVec
}

// NewVecTimer creates a new VecTimer. The provided ObserverVec is used to observe a
// duration in seconds. Timer is usually used to time a function call in the
// following way:
//    func TimeMe() {
//        timer := NewVecTimer(myHistogramVec)
//        // Do actual work.
//        timer.ObserveDurationWithLabelValues("label1", "label2")
//    }
func NewVecTimer(v prometheus.ObserverVec) *VecTimer {
	return &VecTimer{
		begin: time.Now(),
		vec:   v,
	}
}

// ObserveWithLabelValues records the duration passed since the VecTimer was created.
// It derives an Observer from the ObserverVec passed during construction
// from the provided labels. It calls the Observe method of the Observer
// with the duration in seconds as an argument. The observed duration is also returned.
//
// Note that this method is only guaranteed to never observe negative durations
// if used with Go1.9+.
func (t *VecTimer) ObserveWithLabelValues(labels ...string) time.Duration {
	d := time.Since(t.begin)
	if t.vec != nil {
		t.vec.WithLabelValues(labels...).Observe(d.Seconds())
	}
	return d
}

// ObserveWith records the duration passed since the VecTimer was created.
// It derives an Observer from the ObserverVec passed during construction
// from the provided labels. It calls the Observe method of the Observer
// with the duration in seconds as an argument. The observed duration is also returned.
//
// Note that this method is only guaranteed to never observe negative durations
// if used with Go1.9+.
func (t *VecTimer) ObserveWith(labels prometheus.Labels) time.Duration {
	d := time.Since(t.begin)
	if t.vec != nil {
		t.vec.With(labels).Observe(d.Seconds())
	}
	return d
}

// ObserveErr sets a label equal to LabelStatus based on the err value and records the
// duration passed since the VecTimer was created.
// The observed duration is also returned.
//
// Note that this method is only guaranteed to never observe negative durations
// if used with Go1.9+.
func (t *VecTimer) ObserveErr(err error) time.Duration {
	d := time.Since(t.begin)
	if t.vec != nil {
		var status string
		if err == nil {
			status = "success"
		} else {
			status = "error"
		}
		t.vec.With(prometheus.Labels{LabelStatus: status}).Observe(d.Seconds())
	}
	return d
}
