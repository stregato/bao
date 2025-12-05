package core

import "sync"

type Registry[T any] struct {
	handles   map[int64]T
	count     int64
	countSync sync.Mutex
}

func (h *Registry[T]) Add(v T) int64 {
	h.countSync.Lock()
	defer h.countSync.Unlock()
	h.count++
	if h.handles == nil {
		h.handles = make(map[int64]T)
	}
	h.handles[h.count] = v
	return h.count
}

func (h *Registry[T]) Get(i int64) (T, error) {
	v, ok := h.handles[i]
	if !ok {
		return v, Errorw("handle %d not found", i)
	}
	return v, nil
}

func (h *Registry[T]) Remove(i int64) {
	delete(h.handles, i)
}
