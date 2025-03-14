package pointer

func Of[T any](v T) *T {
	return &v
}
