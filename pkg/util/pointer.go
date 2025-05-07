package util

func Value[T any](v *T) T {
	if v == nil {
		var zero T
		return zero
	}
	return *v
}
