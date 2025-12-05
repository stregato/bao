package core

// Apply applies the function f to each item in the slice s and returns a new slice with the results.
// The function f takes an element of type T and returns an element of type U.
func Apply[T any, U any](s []T, f func(T) (res U, ok bool)) []U {
	var result []U
	for _, v := range s {
		r, ok := f(v)
		if ok {
			result = append(result, r)
		}
	}
	return result
}

func Diff[T comparable](a, b []T) (onlyA, onlyB, both []T) {
	aSet := NewSet(a...)
	bSet := NewSet(b...)

	for v := range aSet {
		if bSet[v] {
			both = append(both, v)
		} else {
			onlyA = append(onlyA, v)
		}
	}
	for v := range bSet {
		if !aSet[v] {
			onlyB = append(onlyB, v)
		}
	}

	return
}

// CopyMap copies a map of any type.
func CopyMap[K comparable, V any](originalMap map[K]V) map[K]V {
	copiedMap := make(map[K]V, len(originalMap))
	for key, value := range originalMap {
		copiedMap[key] = value
	}
	return copiedMap
}

// CopySlice copies a slice of any type.
func CopySlice[T any](originalSlice []T) []T {
	copiedSlice := make([]T, len(originalSlice))
	copy(copiedSlice, originalSlice)
	return copiedSlice
}
