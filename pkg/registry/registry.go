package registry

import "sync"

// K is a typed key.
type K[T any] string

func New() *Registry {
	return &Registry{
		values: make(map[string]any),
	}
}

type Registry struct {
	mu     sync.RWMutex
	values map[string]any
}

func (r *Registry) Set(k string, v any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.values[k] = v
}

func (r *Registry) Get(k string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	v, ok := r.values[k]
	return v, ok
}

// GetAs retrieves a value from the registry as the specified type. UB for bad keys/types.
func GetAs[T any](r *Registry, k K[T]) T {
	if v, ok := r.Get(string(k)); ok {
		if vt, ok := v.(T); ok {
			return vt
		}
	}
	return *new(T)
}

func SetAs[T any](r *Registry, k K[T], v T) {
	r.Set(string(k), v)
}
