package dag

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestGraphRunHonoursDependencies(t *testing.T) {
	graph := New[string]()
	for _, node := range []struct {
		id   string
		deps []string
	}{
		{id: "build", deps: []string{"parse", "load"}},
		{id: "parse", deps: []string{"load"}},
		{id: "load"},
		{id: "emit", deps: []string{"build"}},
	} {
		if err := graph.Add(node.id, node.deps, node.id); err != nil {
			t.Fatal(err)
		}
	}

	var order []string
	if err := graph.Run(context.Background(), 1, func(ctx context.Context, value string) error {
		order = append(order, value)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	assertBefore(t, order, "load", "parse")
	assertBefore(t, order, "parse", "build")
	assertBefore(t, order, "build", "emit")
}

func TestGraphRunReportsUnresolvedDependency(t *testing.T) {
	graph := New[string]()
	if err := graph.Add("build", []string{"parse"}, "build"); err != nil {
		t.Fatal(err)
	}

	err := graph.Run(context.Background(), 1, func(ctx context.Context, value string) error {
		return nil
	})
	if !errors.Is(err, ErrUnresolvedDependency) {
		t.Fatalf("err = %v, want ErrUnresolvedDependency", err)
	}
}

func TestGraphRunReportsCircularDependency(t *testing.T) {
	graph := New[string]()
	if err := graph.Add("a", []string{"b"}, "a"); err != nil {
		t.Fatal(err)
	}
	if err := graph.Add("b", []string{"a"}, "b"); err != nil {
		t.Fatal(err)
	}

	err := graph.Run(context.Background(), 2, func(ctx context.Context, value string) error {
		return nil
	})
	if !errors.Is(err, ErrCircularDependency) {
		t.Fatalf("err = %v, want ErrCircularDependency", err)
	}
}

func TestGraphAddRejectsDuplicateAndSelfDependency(t *testing.T) {
	graph := New[string]()
	if err := graph.Add("a", nil, "a"); err != nil {
		t.Fatal(err)
	}
	if err := graph.Add("a", nil, "again"); !errors.Is(err, ErrDuplicateNode) {
		t.Fatalf("duplicate err = %v, want ErrDuplicateNode", err)
	}
	if err := graph.Add("b", []string{"b"}, "b"); !errors.Is(err, ErrSelfDependency) {
		t.Fatalf("self dependency err = %v, want ErrSelfDependency", err)
	}
	if err := graph.AddDeps("missing", nil); !errors.Is(err, ErrMissingNode) {
		t.Fatalf("missing node err = %v, want ErrMissingNode", err)
	}
}

func assertBefore(t *testing.T, values []string, before, after string) {
	t.Helper()
	beforeIndex := slices.Index(values, before)
	afterIndex := slices.Index(values, after)
	if beforeIndex == -1 || afterIndex == -1 || beforeIndex >= afterIndex {
		t.Fatalf("order = %#v, want %q before %q", values, before, after)
	}
}
