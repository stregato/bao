package core

import (
	"context"
	"sync"
	"time"
)

type asyncResult struct {
	Value any
	Err   error
}

var (
	asyncMu     sync.Mutex
	asyncNextId uint64
	asyncTasks  = make(map[uint64]chan asyncResult)
)

func AsyncFunc[T any](fn func() (T, error)) uint64 {
	asyncMu.Lock()
	asyncNextId++
	hnd := asyncNextId
	asyncTasks[hnd] = make(chan asyncResult, 1)

	go func() {
		v, err := fn()
		asyncTasks[hnd] <- asyncResult{v, err}
	}()
	asyncMu.Unlock()

	return hnd
}

func AsyncWait(hnd uint64, timeout time.Duration) (any, error) {
	asyncMu.Lock()
	ch, ok := asyncTasks[hnd]
	asyncMu.Unlock()
	if !ok {
		return nil, nil
	}
	var res asyncResult
	select {
	case res = <-ch:
		// done
	case <-time.After(timeout):
		return nil, context.DeadlineExceeded
	}
	asyncMu.Lock()
	delete(asyncTasks, hnd)
	asyncMu.Unlock()
	return res.Value, res.Err
}
