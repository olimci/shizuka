package markdown

import (
	"bytes"
	"fmt"
	"strings"

	gm "github.com/yuin/goldmark"
	gmast "github.com/yuin/goldmark/ast"
	gmparse "github.com/yuin/goldmark/parser"
	gmrenderer "github.com/yuin/goldmark/renderer"
	gmtext "github.com/yuin/goldmark/text"
	gmutil "github.com/yuin/goldmark/util"
)

var kindWikilink = gmast.NewNodeKind("Wikilink")

type wikilink struct {
	gmast.BaseInline
	Target string
	Label  string
}

func (n *wikilink) Kind() gmast.NodeKind {
	return kindWikilink
}

func (n *wikilink) Dump(source []byte, level int) {
	gmast.DumpHelper(n, source, level, map[string]string{
		"Target": n.Target,
		"Label":  n.Label,
	}, nil)
}

type wikilinkExtension struct {
	resolver TargetResolver
}

func newWikilinkExtension(resolver TargetResolver) gm.Extender {
	return wikilinkExtension{resolver: resolver}
}

func (e wikilinkExtension) Extend(md gm.Markdown) {
	md.Parser().AddOptions(gmparse.WithInlineParsers(
		gmutil.Prioritized(wikilinkParser{}, 199),
	))
	md.Renderer().AddOptions(gmrenderer.WithNodeRenderers(
		gmutil.Prioritized(wikilinkRenderer{resolver: e.resolver}, 500),
	))
}

type wikilinkParser struct{}

func (p wikilinkParser) Trigger() []byte {
	return []byte{'['}
}

func (p wikilinkParser) Parse(parent gmast.Node, block gmtext.Reader, pc gmparse.Context) gmast.Node {
	if pc.IsInLinkLabel() {
		return nil
	}

	line, segment := block.PeekLine()
	if !bytes.HasPrefix(line, []byte("[[")) {
		return nil
	}

	end := bytes.Index(line[2:], []byte("]]"))
	if end == -1 {
		return nil
	}

	raw := string(line[2 : 2+end])
	target, label, hasLabel := strings.Cut(raw, "|")
	if !hasLabel {
		label = target
	}

	node := &wikilink{
		Target: target,
		Label:  label,
	}
	node.SetPos(segment.Start)
	block.Advance(2 + end + 2)
	return node
}

type wikilinkRenderer struct {
	resolver TargetResolver
}

func (r wikilinkRenderer) RegisterFuncs(reg gmrenderer.NodeRendererFuncRegisterer) {
	reg.Register(kindWikilink, r.render)
}

func (r wikilinkRenderer) render(w gmutil.BufWriter, source []byte, node gmast.Node, entering bool) (gmast.WalkStatus, error) {
	if !entering {
		return gmast.WalkSkipChildren, nil
	}

	n := node.(*wikilink)
	if n.Label == "" {
		return gmast.WalkStop, fmt.Errorf("wikilink %q: label is empty", n.Target)
	}
	href, err := r.resolver(n.Target)
	if err != nil {
		return gmast.WalkStop, fmt.Errorf("wikilink %q: %w", n.Target, err)
	}

	_, _ = w.WriteString(`<a href="`)
	_, _ = w.Write(gmutil.EscapeHTML(gmutil.URLEscape([]byte(href), true)))
	_, _ = w.WriteString(`">`)
	_, _ = w.Write(gmutil.EscapeHTML([]byte(n.Label)))
	_, _ = w.WriteString(`</a>`)
	return gmast.WalkSkipChildren, nil
}
