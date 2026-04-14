package internal

func lazySend[T any](ch chan<- T, value T) {
	select {
	case ch <- value:
	default:
	}
}
