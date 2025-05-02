package concurrency

import (
	"sync"
	"testing"
)

func TestOne(t *testing.T) {
	lock := NewKeyLocker[string](&sync.Mutex{})

	lock.Lock("a")
	lock.Unlock("a")
	lock.Lock("a")
}

func TestMany(t *testing.T) {
	lock := NewKeyLocker[string](&sync.Mutex{})

	var wg sync.WaitGroup
	wg.Add(100)

	for range 100 {
		go func() {
			defer wg.Done()
			defer lock.Unlock("a")

			lock.Lock("a")
		}()
	}

	wg.Wait()
}
