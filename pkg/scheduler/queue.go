package scheduler

type chanQueue[T any] struct {
	channal chan T
}

func newChanQueue[T any]() *chanQueue[T] {
	return &chanQueue[T]{
		channal: make(chan T, 10_000),
	}
}

func (q *chanQueue[T]) Push(item T) {
	q.channal <- item
}

func (q *chanQueue[T]) Pop() <-chan T {
	return q.channal
}
