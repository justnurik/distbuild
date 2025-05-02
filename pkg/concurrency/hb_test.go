package concurrency

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSimple(t *testing.T) {
	order := atomic.Int64{}
	hb := NewHappenceBeforeMachine[string]()

	go func() {
		hb.Happen("a", func() { require.True(t, order.CompareAndSwap(0, 1)) })
	}()

	hb.Before("a")
	require.True(t, order.CompareAndSwap(1, 2))
}

func TestSimpleNotOneShot(t *testing.T) {
	order := atomic.Int64{}
	hb := NewHappenceBeforeMachine[string]()

	for i := range 1_000_000 {
		i := int64(i)

		go func() {
			hb.Happen("a", func() { require.True(t, order.CompareAndSwap(2*i, 2*i+1)) })
		}()

		hb.Before("a")
		require.True(t, order.CompareAndSwap(2*i+1, 2*i+2))
	}
}

func TestOneThread(t *testing.T) {
	order := atomic.Int64{}
	hb := NewHappenceBeforeMachine[string]()

	hb.Happen("a", func() { require.True(t, order.CompareAndSwap(0, 1)) })

	hb.Before("a")
	require.True(t, order.CompareAndSwap(1, 2))
}

func TestOneThreadNotOneShot(t *testing.T) {
	order := atomic.Int64{}
	hb := NewHappenceBeforeMachine[string]()

	hb.Happen("a", func() { require.True(t, order.CompareAndSwap(0, 1)) })

	hb.Before("a")
	require.True(t, order.CompareAndSwap(1, 2))

	hb.Happen("a", func() { require.True(t, order.CompareAndSwap(2, 3)) })

	hb.Before("a")
	require.True(t, order.CompareAndSwap(3, 4))
}
