package queue

import (
	"sync"
)

type Queue struct {
	work    []func() error
	lock    sync.Mutex
	cond    *sync.Cond
	gotwork chan struct{}
}

func NewQueue() *Queue {
	res := &Queue{
		gotwork: make(chan struct{}),
	}
	res.cond = sync.NewCond(&res.lock)
	return res
}

func (q *Queue) Get() (work func() error, ok bool) {
	q.lock.Lock()
	q.cond.Wait()
	defer q.lock.Unlock()
	if ok = len(q.work) > 0; ok {
		work, q.work = q.work[0], q.work[1:]
	}
	return
}

func (q *Queue) Add(f func() error) {
	q.lock.Lock()
	q.work = append(q.work, f)
	q.cond.Signal()
	q.lock.Unlock()
}

func (q *Queue) Set(f func() error) {
	q.lock.Lock()
	q.work = []func() error{f}
	q.cond.Signal()
	q.lock.Unlock()
}
