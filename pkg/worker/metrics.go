package worker

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type workerMetrics struct {
	wg    sync.WaitGroup
	close chan struct{}

	iowc io.WriteCloser

	duration time.Duration

	totalJob  atomic.Uint64
	activeJob atomic.Int64
}

func newWorkerMetrics(iow io.WriteCloser, duration time.Duration) *workerMetrics {
	w := &workerMetrics{
		close:    make(chan struct{}),
		iowc:     iow,
		duration: duration,
	}

	w.wg.Add(1)
	go w.show()

	return w
}

func (w *workerMetrics) show() {
	defer w.wg.Done()

	for {
		select {
		case <-w.close:
			return
		case <-time.After(w.duration):
		}

		fmt.Fprintln(w.iowc, strings.Repeat("█", int(w.activeJob.Load())))
	}

}

func (w *workerMetrics) scheduleTask() {
	w.activeJob.Add(1)
	w.totalJob.Add(1)
}

func (w *workerMetrics) doneTask() {
	w.activeJob.Add(-1)
	w.totalJob.Add(1)
}

func (w *workerMetrics) stop() {
	close(w.close)
	w.wg.Wait()
	w.iowc.Close()
}
