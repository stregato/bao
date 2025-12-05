package core

import (
	"encoding/json"
)

type Set[T comparable] map[T]bool

func NewSet[T comparable](s ...T) Set[T] {
	result := make(map[T]bool)

	for _, v := range s {
		result[v] = true
	}
	return result
}

func (s Set[T]) Add(v T) bool {
	if s[v] {
		return false
	}
	s[v] = true
	return true
}

func (s Set[T]) Remove(v T) bool {
	if !s[v] {
		return false
	}
	delete(s, v)
	return true
}

func (s Set[T]) Contains(v T) bool {
	return s[v]
}

func (s Set[T]) RemovedItems() Set[T] {
	result := make(Set[T])

	for v := range s {
		if !s[v] {
			result[v] = true
		}
	}
	return result
}

func (s Set[T]) Slice() []T {
	var result []T
	for v := range s {
		result = append(result, v)
	}
	return result
}

func (s *Set[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Slice())
}

func (s *Set[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	*s = NewSet(items...)

	return nil
}
