package init_ui

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/bubbles/list"
	"github.com/olimci/shizuka/pkg/scaffold"
)

type templateItem struct {
	title string
	desc  string
	tmpl  *scaffold.Template
}

func (t templateItem) Title() string       { return t.title }
func (t templateItem) Description() string { return t.desc }
func (t templateItem) FilterValue() string { return t.title }

func buildTemplateList(coll *scaffold.Collection) list.Model {
	items := make([]list.Item, 0, len(coll.Templates))
	for _, tmpl := range coll.Templates {
		desc := tmpl.Config.Metadata.Description
		if desc == "" {
			desc = tmpl.Config.Metadata.Slug
		}
		items = append(items, templateItem{
			title: tmpl.Config.Metadata.Name,
			desc:  desc,
			tmpl:  tmpl,
		})
	}

	delegate := list.NewDefaultDelegate()
	styles := initStyles()
	delegate.Styles.SelectedTitle = styles.listSelectedTitle
	delegate.Styles.SelectedDesc = styles.listSelectedDesc
	delegate.Styles.NormalTitle = styles.listNormalTitle
	delegate.Styles.NormalDesc = styles.listNormalDesc

	l := list.New(items, delegate, 0, 0)
	l.Title = "Select a template:"
	l.Styles.Title = styles.listTitle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	return l
}

func resolveTemplate(coll *scaffold.Collection, selected string) (*scaffold.Template, error) {
	for _, tmpl := range coll.Templates {
		if tmpl.Config.Metadata.Slug == selected || tmpl.Config.Metadata.Name == selected {
			return tmpl, nil
		}
	}

	return nil, fmt.Errorf("template %s not found in collection", selected)
}

func selectTemplateInList(l *list.Model, selected string) {
	for i, item := range l.Items() {
		tmplItem, ok := item.(templateItem)
		if !ok || tmplItem.tmpl == nil {
			continue
		}
		if tmplItem.tmpl.Config.Metadata.Slug == selected || tmplItem.tmpl.Config.Metadata.Name == selected {
			l.Select(i)
			return
		}
	}
}

func sortedVarKeys(tmpl *scaffold.Template) []string {
	keys := make([]string, 0, len(tmpl.Config.Variables))
	for key := range tmpl.Config.Variables {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
