package queue

import (
	"sync"
)

type Queue struct {
	work    []func() error
	lock    sync.Mutex
	gotwork chan struct{}
}

func NewQueue() *Queue {
	res := &Queue{
		gotwork: make(chan struct{}),
	}
	return res
}

func (q *Queue) Get() (work func() error, wait chan struct{}) {
	q.lock.Lock()
	defer q.lock.Unlock()
	if len(q.work) > 0 {
		work, q.work = q.work[0], q.work[1:]
	} else {
		wait = make(chan struct{})
		q.gotwork = wait
	}
	return
}

func (q *Queue) Add(f func() error) {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.work = append(q.work, f)
	q.signalWork()
}

func (q *Queue) Set(f func() error) {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.work = []func() error{f}
	q.signalWork()
}

func (q *Queue) signalWork() {
	if q.gotwork != nil {
		close(q.gotwork)
		q.gotwork = nil
	}
}
