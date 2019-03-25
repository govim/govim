package queue

import (
	"sync"
)

type Queue struct {
	work    []func()
	lock    sync.Mutex
	gotwork chan struct{}
	dying   <-chan struct{}
}

func NewQueue(dying <-chan struct{}) *Queue {
	res := &Queue{
		gotwork: make(chan struct{}),
		dying:   dying,
	}
	return res
}

func (q *Queue) Run() error {
	for {
		select {
		case <-q.gotwork:
		case <-q.dying:
			return nil
		}
		for {
			var work func()
			q.lock.Lock()
			if len(q.work) == 0 {
				q.lock.Unlock()
				break
			}
			work, q.work = q.work[0], q.work[1:]
			q.lock.Unlock()
			work()
		}
	}
}

func (q *Queue) Add(f func()) {
	q.lock.Lock()
	q.work = append(q.work, f)
	go q.signalWork()
	q.lock.Unlock()
}

func (q *Queue) Set(f func()) {
	q.lock.Lock()
	q.work = []func(){f}
	go q.signalWork()
	q.lock.Unlock()
}

func (q *Queue) signalWork() {
	q.gotwork <- struct{}{}
}
