package dag

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"golang.org/x/sync/errgroup"
)

var (
	ErrDuplicateNode        = errors.New("duplicate node")
	ErrMissingNode          = errors.New("missing node")
	ErrSelfDependency       = errors.New("self dependency")
	ErrUnresolvedDependency = errors.New("unresolved dependency")
	ErrCircularDependency   = errors.New("circular dependency")
	ErrNilNodeFn            = errors.New("nil node function")
)

// New creates an empty graph.
func New[T any]() *Graph[T] {
	return &Graph[T]{
		nodes: make(map[string]T),
		edges: make(map[string]map[string]bool),
	}
}

type Graph[T any] struct {
	nodes map[string]T
	edges map[string]map[string]bool
}

// Len returns the number of nodes in the graph.
func (g *Graph[T]) Len() int {
	return len(g.nodes)
}

type NodeFn[T any] func(ctx context.Context, value T) error

// Add registers a node and the nodes it depends on.
func (g *Graph[T]) Add(id string, deps []string, value T) error {
	if _, exists := g.nodes[id]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateNode, id)
	}
	if slices.Contains(deps, id) {
		return fmt.Errorf("%w: %s", ErrSelfDependency, id)
	}

	g.nodes[id] = value
	g.addDeps(id, deps)
	return nil
}

// AddDeps adds dependencies to an existing node.
func (g *Graph[T]) AddDeps(id string, deps []string) error {
	if _, exists := g.nodes[id]; !exists {
		return fmt.Errorf("%w: %s", ErrMissingNode, id)
	}
	if slices.Contains(deps, id) {
		return fmt.Errorf("%w: %s", ErrSelfDependency, id)
	}

	g.addDeps(id, deps)
	return nil
}

func (g *Graph[T]) addDeps(id string, deps []string) {
	edges := g.edges[id]
	if edges == nil {
		edges = make(map[string]bool)
		g.edges[id] = edges
	}

	for _, dep := range deps {
		edges[dep] = true
	}
}

func (g *Graph[T]) compile() (map[string][]string, map[string]int, error) {
	adj := make(map[string][]string, len(g.nodes))
	deg := make(map[string]int, len(g.nodes))
	for id := range g.nodes {
		deg[id] = 0
	}

	for id, deps := range g.edges {
		if _, exists := g.nodes[id]; !exists {
			return nil, nil, fmt.Errorf("%w: %s", ErrMissingNode, id)
		}
		for dep := range deps {
			if id == dep {
				return nil, nil, fmt.Errorf("%w: %s", ErrSelfDependency, id)
			}
			if _, exists := g.nodes[dep]; !exists {
				return nil, nil, fmt.Errorf("%w: %s", ErrUnresolvedDependency, dep)
			}

			deg[id]++
			adj[dep] = append(adj[dep], id)
		}
	}

	return adj, deg, nil
}

func (g *Graph[T]) Run(ctx context.Context, workers int, fn NodeFn[T]) error {
	if fn == nil {
		return ErrNilNodeFn
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(g.nodes) == 0 {
		return nil
	}

	adj, deg, err := g.compile()
	if err != nil {
		return err
	}

	ready := make([]string, 0, len(deg))
	for id, d := range deg {
		if d == 0 {
			ready = append(ready, id)
		}
	}
	if len(ready) == 0 {
		return fmt.Errorf("%w: %v", ErrCircularDependency, stuck(deg))
	}

	if workers <= 0 || workers > len(g.nodes) {
		workers = len(g.nodes)
	}

	readyCh := make(chan string, len(g.nodes))
	for _, id := range ready {
		readyCh <- id
	}

	state := struct {
		mu      sync.Mutex
		active  int
		queued  int
		remain  int
		stuck   error
		closed  bool
		closeCh func()
	}{
		queued: len(ready),
		remain: len(g.nodes),
	}
	state.closeCh = func() {
		if state.closed {
			return
		}
		close(readyCh)
		state.closed = true
	}

	group, groupCtx := errgroup.WithContext(ctx)
	for range workers {
		group.Go(func() error {
			for {
				select {
				case <-groupCtx.Done():
					return nil
				case id, ok := <-readyCh:
					if !ok {
						return nil
					}

					state.mu.Lock()
					state.active++
					state.queued--
					state.mu.Unlock()

					if err := fn(groupCtx, g.nodes[id]); err != nil {
						return err
					}

					state.mu.Lock()
					state.active--
					state.remain--
					for _, next := range adj[id] {
						deg[next]--
						if deg[next] == 0 {
							readyCh <- next
							state.queued++
						}
					}

					switch {
					case state.remain == 0:
						state.closeCh()
					case state.active == 0 && state.queued == 0:
						state.stuck = fmt.Errorf("%w: %v", ErrCircularDependency, stuck(deg))
						state.closeCh()
					}
					state.mu.Unlock()
				}
			}
		})
	}

	if err := group.Wait(); err != nil {
		return err
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if state.stuck != nil {
		return state.stuck
	}
	return ctx.Err()
}

func stuck(deg map[string]int) []string {
	stuck := make([]string, 0)
	for id, d := range deg {
		if d != 0 {
			stuck = append(stuck, id)
		}
	}
	return stuck
}
