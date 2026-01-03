package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/olimci/prompter"
	"github.com/olimci/shizuka/pkg/scaffold"
)

type initInteractiveParams struct {
	Template   *scaffold.Template
	Collection *scaffold.Collection
	Selected   string
	Target     string
}

type initInteractiveResult struct {
	Template  *scaffold.Template
	Target    string
	Variables map[string]any
	Cancelled bool
}

func runInitInteractive(ctx context.Context, params initInteractiveParams) (*initInteractiveResult, error) {
	result := &initInteractiveResult{
		Target:    params.Target,
		Variables: map[string]any{},
	}

	completed := false
	err := prompter.Start(func(p *prompter.Prompter) error {
		chosen, err := promptInitTemplate(p, params.Template, params.Collection, params.Selected)
		if err != nil {
			return err
		}
		result.Template = chosen

		target, err := promptInitTarget(p, params.Target)
		if err != nil {
			return err
		}
		result.Target = target

		vars, err := promptInitVariables(p, chosen)
		if err != nil {
			return err
		}
		result.Variables = vars

		completed = true
		return nil
	}, prompter.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	if !completed {
		result.Cancelled = true
	}

	return result, nil
}

func promptInitTemplate(p *prompter.Prompter, tmpl *scaffold.Template, coll *scaffold.Collection, selected string) (*scaffold.Template, error) {
	if tmpl != nil {
		return tmpl, nil
	}
	if coll == nil {
		return nil, ErrTemplateNotFound
	}

	selected = strings.TrimSpace(selected)
	if selected != "" {
		if resolved := findTemplateInCollection(coll, selected); resolved != nil {
			return resolved, nil
		}
		return nil, fmt.Errorf("template %s not found in collection", selected)
	}

	var defaultLabel string
	defaultKey := strings.TrimSpace(coll.Config.Templates.Default)

	options := make([]string, 0, len(coll.Templates))
	byLabel := make(map[string]*scaffold.Template, len(coll.Templates))

	for _, item := range coll.Templates {
		if item == nil {
			continue
		}

		label := templateOptionLabel(item)
		uniqueLabel := label
		for i := 2; byLabel[uniqueLabel] != nil; i++ {
			uniqueLabel = fmt.Sprintf("%s #%d", label, i)
		}

		options = append(options, uniqueLabel)
		byLabel[uniqueLabel] = item

		if defaultLabel == "" && defaultKey != "" && templateMatchesKey(item, defaultKey) {
			defaultLabel = uniqueLabel
		}
	}

	if len(options) == 0 {
		return nil, fmt.Errorf("no templates found in collection")
	}

	var (
		chosenLabel string
		err         error
	)

	if defaultLabel != "" {
		chosenLabel, err = p.AwaitSelectDefault("Select a template:", options, defaultLabel)
	} else {
		chosenLabel, err = p.AwaitSelect("Select a template:", options)
	}
	if err != nil {
		return nil, err
	}

	chosen := byLabel[chosenLabel]
	if chosen == nil {
		return nil, fmt.Errorf("selected template not found")
	}

	return chosen, nil
}

func promptInitTarget(p *prompter.Prompter, target string) (string, error) {
	defaultTarget := strings.TrimSpace(target)
	if defaultTarget == "" {
		defaultTarget = "."
	}

	value, err := p.AwaitInput(
		prompter.WithInputPrompt(fmt.Sprintf("Target directory (blank for %s): ", defaultTarget)),
	)
	if err != nil {
		return "", err
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return defaultTarget, nil
	}

	return value, nil
}

func promptInitVariables(p *prompter.Prompter, tmpl *scaffold.Template) (map[string]any, error) {
	if tmpl == nil {
		return nil, fmt.Errorf("no template selected")
	}

	varKeys := make([]string, 0, len(tmpl.Config.Variables))
	for key := range tmpl.Config.Variables {
		varKeys = append(varKeys, key)
	}
	sort.Strings(varKeys)

	vars := make(map[string]any, len(varKeys))
	for _, key := range varKeys {
		def := tmpl.Config.Variables[key]
		name := variablePromptName(key, def)

		prompt := fmt.Sprintf("%s: ", name)
		if def.Default != "" {
			prompt = fmt.Sprintf("%s (blank for %q): ", name, def.Default)
		}

		opts := []prompter.InputOption{
			prompter.WithInputPrompt(prompt),
		}
		if def.Description != "" {
			opts = append(opts, prompter.WithInputPlaceholder(def.Description))
		}

		value, err := p.AwaitInput(opts...)
		if err != nil {
			return nil, err
		}

		value = strings.TrimSpace(value)
		if value == "" && def.Default != "" {
			value = def.Default
		}

		vars[key] = value
	}

	return vars, nil
}

func findTemplateInCollection(coll *scaffold.Collection, key string) *scaffold.Template {
	if coll == nil {
		return nil
	}
	key = strings.TrimSpace(key)
	for _, tmpl := range coll.Templates {
		if tmpl == nil {
			continue
		}
		if templateMatchesKey(tmpl, key) {
			return tmpl
		}
	}
	return nil
}

func templateMatchesKey(tmpl *scaffold.Template, key string) bool {
	if tmpl == nil {
		return false
	}
	return tmpl.Config.Metadata.Slug == key || tmpl.Config.Metadata.Name == key
}

func templateOptionLabel(tmpl *scaffold.Template) string {
	name := strings.TrimSpace(tmpl.Config.Metadata.Name)
	slug := strings.TrimSpace(tmpl.Config.Metadata.Slug)
	desc := strings.TrimSpace(tmpl.Config.Metadata.Description)

	if name == "" {
		name = slug
	}

	label := name
	if slug != "" && slug != name {
		label = fmt.Sprintf("%s (%s)", name, slug)
	}
	if desc != "" {
		label = fmt.Sprintf("%s - %s", label, desc)
	}

	return label
}

func variablePromptName(key string, def scaffold.TemplateCfgVar) string {
	name := strings.TrimSpace(def.Name)
	if name == "" {
		return key
	}
	if name == key {
		return key
	}
	return fmt.Sprintf("%s (%s)", name, key)
}
