package drainer

// emitWaiter collects output from
// github.com/olebedev/emitter Emitter.Emit()
// and has a method to Wait on all channels.
type emitWaiter []<-chan struct{}

// Wait for all events to be emitted.
func (ew emitWaiter) Wait() {
	for _, c := range ew {
		<-c
	}
}
