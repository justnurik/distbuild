package concurrency

import (
	"sync"
	"sync/atomic"
)

const (
	waiting = iota
	nothing
	happened
)

type HappenceBeforeMachine[K comparable] struct {
	mu sync.Mutex

	waitMap map[K]*chan struct{}
	wasMap  map[K]*atomic.Int32
}

func NewHappenceBeforeMachine[K comparable]() *HappenceBeforeMachine[K] {
	return &HappenceBeforeMachine[K]{
		waitMap: make(map[K]*chan struct{}),
		wasMap:  make(map[K]*atomic.Int32),
	}
}

func (h *HappenceBeforeMachine[K]) create(key K) (*chan struct{}, *atomic.Int32) {
	h.mu.Lock()
	defer h.mu.Unlock()

	wait, exist := h.waitMap[key]
	if !exist {
		waitChan := make(chan struct{})
		wait = &waitChan
		h.waitMap[key] = wait
	}

	was, exist := h.wasMap[key]
	if !exist {
		wasTmp := atomic.Int32{}
		wasTmp.Store(nothing)

		was = &wasTmp
		h.wasMap[key] = was
	}

	return wait, was
}

func (h *HappenceBeforeMachine[K]) Happen(key K, event func()) {
	wait, was := h.create(key)

	event()
	if was.Add(1) == nothing {
		close(*wait)
	}
}

func (h *HappenceBeforeMachine[K]) Before(key K) {
	wait, was := h.create(key)

	if was.Add(-1) == waiting {
		<-(*wait)
		(*wait) = make(chan struct{})
	}
}
