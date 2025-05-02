package concurrency

import "sync"

type KeyLocker[K comparable] struct {
	mu   sync.Locker
	lock map[K]chan struct{}
}

func NewKeyLocker[K comparable](mu sync.Locker) *KeyLocker[K] {
	return &KeyLocker[K]{
		mu:   mu,
		lock: make(map[K]chan struct{}),
	}
}

func (k *KeyLocker[K]) Lock(key K) {
	k.mu.Lock()

	if _, exist := k.lock[key]; !exist {
		k.lock[key] = make(chan struct{}, 1)
	}
	local := k.lock[key]
	k.mu.Unlock()

	local <- struct{}{}
}

func (k *KeyLocker[K]) Unlock(key K) {
	k.mu.Lock()

	if _, exist := k.lock[key]; !exist {
		return
	}
	local := k.lock[key]
	k.mu.Unlock()

	<-local
}
