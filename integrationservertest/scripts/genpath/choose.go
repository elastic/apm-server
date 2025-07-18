package main

import (
	"math/rand/v2"
)

func choose[T any](list []T, n int) []T {
	if n < 0 {
		panic("choose: n cannot be negative")
	}

	if n == 1 {
		return []T{list[rand.IntN(len(list))]}
	}

	if n > len(list) {
		n = len(list)
	}
	indices := rand.Perm(len(list))
	result := make([]T, 0, n)
	for i, idx := range indices {
		if i >= n {
			break
		}
		result = append(result, list[idx])
	}
	return result
}
