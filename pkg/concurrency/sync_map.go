package concurrency

import (
	"sync"
)

type SyncMap[K comparable, V any] struct {
	data map[K]V
	mu   sync.RWMutex
}

func NewSyncMap[K comparable, V any](cap int) *SyncMap[K, V] {
	return &SyncMap[K, V]{
		data: make(map[K]V, cap),
	}
}

func (s *SyncMap[K, V]) Store(key K, val V) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[key] = val
}

func (s *SyncMap[K, V]) Load(key K) (V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exist := s.data[key]
	return val, exist
}

func (s *SyncMap[K, V]) LoadUnsafe(key K) V {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.data[key]
}
