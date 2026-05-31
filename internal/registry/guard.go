package registry

func R[T any](key K[T]) Lock {
	return Lock{
		key: string(key),
	}
}

func RX[T any](key K[T]) Lock {
	return Lock{
		key:      string(key),
		optional: true,
	}
}

func W[T any](key K[T]) Lock {
	return Lock{
		write: true,
		key:   string(key),
	}
}

type Lock struct {
	key      string
	write    bool
	optional bool
	cell     *cell
}

type Guard struct {
	locks []Lock
	s     *Scoped
}

func (g *Guard) Close() {
	if g.locks == nil {
		return
	}

	for i := range g.locks {
		lock := g.locks[len(g.locks)-1-i]
		if lock.cell == nil {
			continue
		}
		if lock.write {
			lock.cell.mu.Unlock()
		} else {
			lock.cell.mu.RUnlock()
		}
	}
	g.locks = nil
	g.s.scope = nil
}
