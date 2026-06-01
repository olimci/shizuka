package markdown

import (
	"fmt"
	"html/template"
	"strings"

	gm "github.com/yuin/goldmark"
	gmast "github.com/yuin/goldmark/ast"
	gmtext "github.com/yuin/goldmark/text"
)

type Document struct {
	Body     template.HTML
	Sections []template.HTML
	ToC      []ToCEntry
}

type ToCEntry struct {
	Level int
	ID    string
	Text  string
}

func Render(md gm.Markdown, sourcePath, rawBody string) (Document, error) {
	source := []byte(rawBody)
	doc := md.Parser().Parse(gmtext.NewReader(source))
	toc := collectToC(source, doc)
	body, err := renderNode(md, sourcePath, source, doc)
	if err != nil {
		return Document{}, err
	}
	sections, err := renderSections(md, sourcePath, source)
	if err != nil {
		return Document{}, err
	}
	return Document{
		Body:     body,
		Sections: sections,
		ToC:      toc,
	}, nil
}

func renderNode(md gm.Markdown, sourcePath string, source []byte, node gmast.Node) (template.HTML, error) {
	var buf strings.Builder
	if err := md.Renderer().Render(&buf, source, node); err != nil {
		return "", fmt.Errorf("render markdown %q: %w", sourcePath, err)
	}
	return template.HTML(buf.String()), nil
}

func collectToC(source []byte, doc gmast.Node) []ToCEntry {
	var toc []ToCEntry
	if err := gmast.Walk(doc, func(node gmast.Node, entering bool) (gmast.WalkStatus, error) {
		if !entering || node.Kind() != gmast.KindHeading {
			return gmast.WalkContinue, nil
		}

		heading := node.(*gmast.Heading)
		entry := ToCEntry{
			Level: heading.Level,
			Text:  string(heading.Text(source)),
		}
		if id, ok := heading.AttributeString("id"); ok {
			entry.ID = string(id.([]byte))
		}
		toc = append(toc, entry)
		return gmast.WalkSkipChildren, nil
	}); err != nil {
		panic(err)
	}
	return toc
}

func renderSections(md gm.Markdown, sourcePath string, source []byte) ([]template.HTML, error) {
	doc := md.Parser().Parse(gmtext.NewReader(source))
	var sections []template.HTML
	section := gmast.NewDocument()
	for node := doc.FirstChild(); node != nil; {
		next := node.NextSibling()
		doc.RemoveChild(doc, node)
		if node.Kind() == gmast.KindThematicBreak {
			body, err := renderNode(md, sourcePath, source, section)
			if err != nil {
				return nil, err
			}
			sections = append(sections, body)
			section = gmast.NewDocument()
			node = next
			continue
		}

		section.AppendChild(section, node)
		node = next
	}

	body, err := renderNode(md, sourcePath, source, section)
	if err != nil {
		return nil, err
	}
	return append(sections, body), nil
}
