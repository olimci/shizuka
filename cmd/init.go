package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/olimci/shizuka/cmd/embed"
	"github.com/olimci/shizuka/cmd/ui/init_ui"
	"github.com/olimci/shizuka/pkg/scaffold"
	"github.com/urfave/cli/v3"
)

var (
	ErrorFailedToLoadTemplate = errors.New("failed to load template")
	ErrTemplateNotFound       = errors.New("no template found")
)

func runInit(ctx context.Context, cmd *cli.Command) error {
	source := ""
	target := "."

	switch cmd.NArg() {
	case 0:
	case 1:
		source = cmd.Args().Get(0)
	case 2:
		source = cmd.Args().Get(0)
		target = cmd.Args().Get(1)
	default:
		return fmt.Errorf("too many arguments!")
	}

	// flags
	selected := cmd.String("template") // selected template
	force := cmd.Bool("force")         // force overwrite
	listOnly := cmd.Bool("list")
	listVars := cmd.Bool("list-vars")

	tmpl, coll, close, err := loadTemplate(ctx, source)
	if err != nil {
		return err
	}
	defer close()

	if listOnly && listVars {
		return fmt.Errorf("--list and --list-vars cannot be used together")
	}

	if listOnly {
		return printTemplateList(tmpl, coll)
	}

	if listVars {
		chosen, err := resolveTemplateFromSource(tmpl, coll, selected)
		if err != nil {
			return err
		}
		return printTemplateVars(chosen)
	}

	result, err := init_ui.Run(ctx, init_ui.Params{
		Template:   tmpl,
		Collection: coll,
		Selected:   selected,
		Target:     target,
		Force:      force,
	})
	if err != nil {
		return err
	}
	if result.Cancelled {
		return nil
	}

	if result.Template == nil {
		return fmt.Errorf("no template selected")
	}

	buildResult, err := result.Template.Build(ctx, result.Target,
		scaffold.WithForce(result.Force),
		scaffold.WithVariables(result.Variables),
	)
	if err != nil {
		return err
	}

	fmt.Print(renderBuildResult(buildResult, result.Target))

	return nil
}

func loadTemplate(ctx context.Context, source string) (tmpl *scaffold.Template, coll *scaffold.Collection, close func(), err error) {
	switch source {
	case "", "default":
		tmpl, coll, err = scaffold.LoadFS(ctx, embed.Scaffold, "scaffold")
	default:
		tmpl, coll, err = scaffold.Load(ctx, source)
	}

	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w: %w", ErrorFailedToLoadTemplate, err)
	} else if tmpl == nil && coll == nil {
		return nil, nil, nil, ErrTemplateNotFound
	} else if tmpl != nil {
		close = sync.OnceFunc(func() {
			_ = tmpl.Close()
		})
	} else if coll != nil {
		close = sync.OnceFunc(func() {
			_ = coll.Close()
		})
	}

	return tmpl, coll, close, nil
}

func renderBuildResult(result *scaffold.BuildResult, target string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Scaffold created in %s\n", target)

	if result == nil {
		return b.String()
	}

	if len(result.DirsCreated) > 0 {
		b.WriteString("\nDirectories:\n")
		for _, dir := range result.DirsCreated {
			fmt.Fprintf(&b, "- %s\n", dir)
		}
	}

	if len(result.FilesCreated) > 0 {
		b.WriteString("\nFiles:\n")
		for _, file := range result.FilesCreated {
			fmt.Fprintf(&b, "- %s\n", file)
		}
	}

	return b.String()
}
