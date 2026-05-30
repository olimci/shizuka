package registry

import (
	"fmt"
	"slices"
	"strings"
	"sync"
)

type cell struct {
	value any
	mu    sync.RWMutex
}

func New() *Registry {
	return &Registry{
		values: make(map[string]*cell),
	}
}

type Registry struct {
	mu     sync.RWMutex
	values map[string]*cell
}

func (r *Registry) Lock(locks ...Lock) (*Guard, *Scoped) {
	scope := make(map[string]*scopedCell)

	r.mu.Lock()
	for i, lock := range locks {
		if _, ex := scope[lock.key]; ex {
			panic(fmt.Sprintf("guard: duplicate key: %q", lock.key))
		}

		c, ok := r.values[lock.key]
		if !ok {
			if !lock.write {
				if lock.optional {
					scope[lock.key] = &scopedCell{}
					continue
				}
				r.mu.Unlock()
				panic(fmt.Sprintf("unknown key: %q", lock.key))
			}
			c = &cell{}
			r.values[lock.key] = c
		}
		locks[i].cell = c
		scope[lock.key] = &scopedCell{
			cell:  c,
			write: lock.write,
		}
	}
	r.mu.Unlock()

	slices.SortFunc(locks, func(a, b Lock) int {
		return strings.Compare(a.key, b.key)
	})

	for _, lock := range locks {
		if lock.cell == nil {
			continue
		}
		if lock.write {
			lock.cell.mu.Lock()
		} else {
			lock.cell.mu.RLock()
		}
	}

	s := &Scoped{scope: scope}
	return &Guard{locks: locks, s: s}, s
}

func (r *Registry) SetAny(k string, v any) bool {
	r.mu.Lock()
	c, ex := r.values[k]
	if !ex {
		r.values[k] = &cell{value: v}
		r.mu.Unlock()
		return true
	}
	r.mu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.value = v

	return true
}

func (r *Registry) GetAny(k string) (any, bool) {
	r.mu.RLock()
	c, ex := r.values[k]
	r.mu.RUnlock()

	if !ex {
		return nil, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	v := c.value

	return v, ex
}

func (r *Registry) DeleteAny(k string) bool {
	r.mu.Lock()
	c, ex := r.values[k]
	if ex {
		delete(r.values, k)
	}
	r.mu.Unlock()

	if !ex {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = nil
	return true
}
