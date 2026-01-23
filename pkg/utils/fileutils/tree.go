package fileutils

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// FSNode represents a file or directory within an FSTree.
// Path is always relative to the WalkTree root; the root node is ".".
type FSNode struct {
	Path  string
	Name  string
	IsDir bool

	Parent   *FSNode
	Children []*FSNode
}

// Child returns the first direct child with the given name.
func (n *FSNode) Child(name string) (*FSNode, bool) {
	if n == nil {
		return nil, false
	}
	for _, c := range n.Children {
		if c != nil && c.Name == name {
			return c, true
		}
	}
	return nil, false
}

// FSTree is a hierarchical representation of a directory tree.
type FSTree struct {
	Root  *FSNode
	Nodes map[string]*FSNode // keyed by relative Path (filepath.Clean), root is "."
}

type TraverseFunc func(node *FSNode, depth int)

func (t *FSTree) Traverse(enter, leave TraverseFunc) {
	if t == nil || t.Root == nil {
		return
	}
	t.traverseNode(t.Root, enter, leave, 0)
}

func (t *FSTree) traverseNode(node *FSNode, enter, leave TraverseFunc, depth int) {
	if node == nil {
		return
	}

	if enter != nil {
		enter(node, depth)
	}

	for _, child := range node.Children {
		t.traverseNode(child, enter, leave, depth+1)
	}

	if leave != nil {
		leave(node, depth)
	}
}

// Node retrieves a node by relative path (e.g. ".", "posts", "posts/hello.md").
func (t *FSTree) Node(rel string) (*FSNode, bool) {
	if t == nil {
		return nil, false
	}
	if t.Nodes == nil {
		return nil, false
	}
	n, ok := t.Nodes[filepath.Clean(rel)]
	return n, ok
}

// WalkTree walks a directory tree and returns a hierarchical representation of files and directories.
// The returned tree always contains a root node with Path ".".
func WalkTree(root string) (*FSTree, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	rootNode := &FSNode{
		Path:  ".",
		Name:  filepath.Base(abs),
		IsDir: true,
	}

	tree := &FSTree{
		Root:  rootNode,
		Nodes: map[string]*FSNode{".": rootNode},
	}

	var ensureDir func(rel string) *FSNode
	ensureDir = func(rel string) *FSNode {
		rel = filepath.Clean(rel)
		if rel == "." {
			return rootNode
		}

		if n, ok := tree.Nodes[rel]; ok {
			return n
		}

		parentRel := filepath.Dir(rel)
		parent := ensureDir(parentRel)

		n := &FSNode{
			Path:   rel,
			Name:   filepath.Base(rel),
			IsDir:  true,
			Parent: parent,
		}

		parent.Children = append(parent.Children, n)
		tree.Nodes[rel] = n
		return n
	}

	err = filepath.WalkDir(abs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return err
		}
		rel = filepath.Clean(rel)
		if rel == "." {
			return nil
		}

		parent := ensureDir(filepath.Dir(rel))

		if existing, ok := tree.Nodes[rel]; ok {
			existing.Name = d.Name()
			existing.IsDir = d.IsDir()
			if existing.Parent == nil {
				existing.Parent = parent
				parent.Children = append(parent.Children, existing)
			}
			return nil
		}

		n := &FSNode{
			Path:   rel,
			Name:   d.Name(),
			IsDir:  d.IsDir(),
			Parent: parent,
		}
		parent.Children = append(parent.Children, n)
		tree.Nodes[rel] = n
		return nil
	})

	if err != nil {
		return nil, err
	}

	return tree, nil
}

// WalkTreeFS walks a filesystem and returns a hierarchical representation of files and directories.
// The returned tree always contains a root node with Path ".".
func WalkTreeFS(fsys fs.FS, root string) (*FSTree, error) {
	root = path.Clean(root)

	rootNode := &FSNode{
		Path:  ".",
		Name:  path.Base(root),
		IsDir: true,
	}
	if root == "." || root == "" {
		rootNode.Name = "."
	}

	tree := &FSTree{
		Root:  rootNode,
		Nodes: map[string]*FSNode{".": rootNode},
	}

	var ensureDir func(rel string) *FSNode
	ensureDir = func(rel string) *FSNode {
		rel = path.Clean(rel)
		if rel == "." {
			return rootNode
		}

		if n, ok := tree.Nodes[rel]; ok {
			return n
		}

		parentRel := path.Dir(rel)
		parent := ensureDir(parentRel)

		n := &FSNode{
			Path:   rel,
			Name:   path.Base(rel),
			IsDir:  true,
			Parent: parent,
		}

		parent.Children = append(parent.Children, n)
		tree.Nodes[rel] = n
		return n
	}

	err := fs.WalkDir(fsys, root, func(current string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if current == root {
			return nil
		}

		rel, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		rel = path.Clean(rel)
		if rel == "." {
			return nil
		}

		parent := ensureDir(path.Dir(rel))

		if existing, ok := tree.Nodes[rel]; ok {
			existing.Name = d.Name()
			existing.IsDir = d.IsDir()
			if existing.Parent == nil {
				existing.Parent = parent
				parent.Children = append(parent.Children, existing)
			}
			return nil
		}

		n := &FSNode{
			Path:   rel,
			Name:   d.Name(),
			IsDir:  d.IsDir(),
			Parent: parent,
		}
		parent.Children = append(parent.Children, n)
		tree.Nodes[rel] = n
		return nil
	})

	if err != nil {
		return nil, err
	}

	return tree, nil
}
