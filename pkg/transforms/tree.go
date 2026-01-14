package transforms

type PageNode struct {
	Page  *Page
	Error error

	Path    string
	URLPath string

	Parent   *PageNode
	Children map[string]*PageNode
}

func (pn *PageNode) AddChild(name string, child *PageNode) bool {
	if pn.Children == nil {
		pn.Children = make(map[string]*PageNode)
	}
	if child, exists := pn.Children[name]; exists && child.Page != nil {
		return false
	}
	if child != nil {
		child.Parent = pn
	}
	pn.Children[name] = child
	return true
}

type PageTree struct {
	Root *PageNode
}

func NewPageTree(root *PageNode) *PageTree {
	if root == nil {
		root = new(PageNode)
	}
	return &PageTree{Root: root}
}

// Pages returns all pages in the tree (depth-first, map iteration order for siblings).
func (pt *PageTree) Pages() []*Page {
	if pt == nil || pt.Root == nil {
		return nil
	}

	pages := make([]*Page, 0)

	var walk func(node *PageNode)
	walk = func(node *PageNode) {
		if node == nil {
			return
		}
		if node.Page != nil {
			pages = append(pages, node.Page)
		}
		for _, child := range node.Children {
			walk(child)
		}
	}

	walk(pt.Root)
	return pages
}

func (pt *PageTree) Nodes() []*PageNode {
	if pt == nil || pt.Root == nil {
		return nil
	}

	nodes := make([]*PageNode, 0)

	var walk func(node *PageNode)
	walk = func(node *PageNode) {
		if node == nil {
			return
		}
		nodes = append(nodes, node)
		for _, child := range node.Children {
			walk(child)
		}
	}

	walk(pt.Root)
	return nodes
}
