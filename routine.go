package via

import (
	"sync/atomic"
	"time"
)

func newOnInterval(ctxDisposedChan, pageStopChan chan struct{}, duration time.Duration, handler func()) func() {
	localInterrupt := make(chan struct{})
	var stopped atomic.Bool

	go func() {
		tkr := time.NewTicker(duration)
		defer tkr.Stop()
		for {
			select {
			case <-ctxDisposedChan:
				return
			case <-pageStopChan:
				return
			case <-localInterrupt:
				return
			case <-tkr.C:
				handler()
			}
		}
	}()

	return func() {
		if stopped.CompareAndSwap(false, true) {
			close(localInterrupt)
		}
	}
}
