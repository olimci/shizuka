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
	if existing, exists := pn.Children[name]; exists {
		if existing.Page != nil {
			return false
		}
		if existing.Parent == nil {
			existing.Parent = pn
		}
		if child != nil && child.Page != nil {
			existing.Page = child.Page
			existing.Error = child.Error
			if child.Path != "" {
				existing.Path = child.Path
			}
			if child.URLPath != "" {
				existing.URLPath = child.URLPath
			}
			child.Page.Tree = existing
		}
		return true
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
