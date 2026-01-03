package cmd

import (
	"fmt"
	"sort"

	"github.com/olimci/shizuka/pkg/scaffold"
)

func resolveTemplateFromSource(tmpl *scaffold.Template, coll *scaffold.Collection, selected string) (*scaffold.Template, error) {
	if tmpl != nil {
		if selected != "" &&
			selected != tmpl.Config.Metadata.Slug &&
			selected != tmpl.Config.Metadata.Name {
			return nil, fmt.Errorf("template %s not found in source", selected)
		}
		return tmpl, nil
	}

	if coll == nil {
		return nil, ErrTemplateNotFound
	}

	if selected == "" {
		if coll.Config.Templates.Default == "" {
			return nil, fmt.Errorf("template must be specified for collections (use --template or --list)")
		}
		selected = coll.Config.Templates.Default
	}

	for _, item := range coll.Templates {
		if item.Config.Metadata.Slug == selected || item.Config.Metadata.Name == selected {
			return item, nil
		}
	}

	return nil, fmt.Errorf("template %s not found in collection", selected)
}

func printTemplateList(tmpl *scaffold.Template, coll *scaffold.Collection) error {
	if tmpl != nil {
		fmt.Printf("Template: %s (%s)\n", tmpl.Config.Metadata.Name, tmpl.Config.Metadata.Slug)
		if tmpl.Config.Metadata.Description != "" {
			fmt.Printf("Description: %s\n", tmpl.Config.Metadata.Description)
		}
		return nil
	}

	if coll == nil {
		return ErrTemplateNotFound
	}

	fmt.Println("Available templates:")
	for _, item := range coll.Templates {
		desc := item.Config.Metadata.Description
		if desc == "" {
			desc = item.Config.Metadata.Slug
		}
		fmt.Printf("- %s (%s): %s\n", item.Config.Metadata.Name, item.Config.Metadata.Slug, desc)
	}
	return nil
}

func printTemplateVars(tmpl *scaffold.Template) error {
	fmt.Printf("Template variables for %s (%s):\n", tmpl.Config.Metadata.Name, tmpl.Config.Metadata.Slug)

	if len(tmpl.Config.Variables) == 0 {
		fmt.Println("- (none)")
		return nil
	}

	keys := make([]string, 0, len(tmpl.Config.Variables))
	for key := range tmpl.Config.Variables {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		def := tmpl.Config.Variables[key]
		line := fmt.Sprintf("- %s", key)
		if def.Name != "" {
			line += fmt.Sprintf(" (%s)", def.Name)
		}
		if def.Default != "" {
			line += fmt.Sprintf(" [default: %s]", def.Default)
		}
		fmt.Println(line)
		if def.Description != "" {
			fmt.Printf("  %s\n", def.Description)
		}
	}

	return nil
}

func applyTemplateVarDefaults(tmpl *scaffold.Template, vars map[string]any) {
	for key, def := range tmpl.Config.Variables {
		if _, ok := vars[key]; ok {
			continue
		}
		if def.Default != "" {
			vars[key] = def.Default
		} else {
			vars[key] = ""
		}
	}
}
