package transforms

func firstNonzero[T comparable](values ...T) T {
	zero := *new(T)
	for _, value := range values {
		if value != zero {
			return value
		}
	}
	return zero
}
