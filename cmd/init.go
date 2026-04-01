package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/olimci/coffee"
	"github.com/olimci/shizuka/cmd/embed"
	"github.com/olimci/shizuka/pkg/scaffold"
	"github.com/urfave/cli/v3"
)

func initFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "output directory",
			Value:   ".",
		},
		&cli.BoolFlag{
			Name:  "list",
			Usage: "list available templates",
		},
		&cli.StringFlag{
			Name:    "template",
			Aliases: []string{"t"},
			Usage:   "template to use",
		},
		&cli.BoolFlag{
			Name:  "list-vars",
			Usage: "list available variables",
		},
		&cli.StringSliceFlag{
			Name:  "var",
			Usage: "template variables (key=value, repeatable)",
		},
		&cli.BoolFlag{
			Name:    "force",
			Aliases: []string{"f"},
			Usage:   "force overwrite existing files",
		},
	}
}

func initCmd() *cli.Command {
	return &cli.Command{
		Name:      "init",
		Usage:     "scaffold a new shizuka site",
		ArgsUsage: "[source]",
		Flags:     initFlags(),
		Action:    runInit,
	}
}

func xInitCmd() *cli.Command {
	return &cli.Command{
		Name:      "xinit",
		Usage:     "scaffold a new shizuka site",
		ArgsUsage: "[source]",
		Flags:     initFlags(),
		Action:    runXInit,
	}
}

func runInit(ctx context.Context, cmd *cli.Command) error {
	if cmd.NArg() > 1 {
		return fmt.Errorf("too many arguments!")
	}
	source := cmd.Args().First()

	flagOutput := cmd.String("output")
	flagTemplate := cmd.String("template")
	flagList := cmd.Bool("list")
	flagListVars := cmd.Bool("list-vars")
	flagVarPairs := cmd.StringSlice("var")
	flagForce := cmd.Bool("force")
	flagVars, err := parseVars(flagVarPairs)
	if err != nil {
		return err
	}

	if flagList && flagListVars {
		return fmt.Errorf("--list and --list-vars cannot be used together!")
	}

	err = coffee.Do(func(ctx context.Context, c *coffee.Coffee) error {
		defer func() {
			_ = c.Clear()
		}()

		tmpl, coll, close, err := initResolve(ctx, source)
		if err != nil {
			return err
		}
		defer close()

		if tmpl == nil && coll != nil {
			if flagList {
				for _, t := range coll.Templates {
					fmt.Println(t.Config.Metadata.Slug)
				}
				return nil
			}

			if flagTemplate != "" {
				if selected := coll.Get(flagTemplate); selected != nil {
					tmpl = selected
				}
			} else {
				selected, err := c.AwaitSelectDefault("select a template:", coll.Config.Templates.Items, coll.Config.Templates.Default)
				if err != nil {
					return err
				}
				tmpl = coll.Get(selected)
			}
		}

		if tmpl == nil {
			return fmt.Errorf("no template found!")
		}

		if flagListVars {
			for _, v := range tmpl.Config.Variables {
				fmt.Printf("%q: %q\n", v.Name, v.Default)
			}
			return nil
		}

		for k, v := range tmpl.Config.Variables {
			_ = c.Logf("variable %s (%s): ", v.Name, v.Description)
			value, err := c.AwaitInput(coffee.WithInputPlaceholder(v.Description))
			if err != nil {
				return err
			}

			flagVars[k] = value
		}

		_ = c.Log("building...")

		res, err := tmpl.Build(ctx, flagOutput, scaffold.BuildOptions{
			Variables: flagVars,
			Force:     flagForce,
		})
		if err != nil {
			return err
		}

		_ = c.Log("Done!")
		_ = c.Logf("Files: %v", res.FilesCreated)
		_ = c.Logf("Dirs:  %v", res.DirsCreated)

		return nil
	}, coffee.WithContext(ctx))
	if err != nil && errors.Is(err, coffee.ErrNonInteractive) {
		return runXInit(ctx, cmd)
	}
	return err
}

func runXInit(ctx context.Context, cmd *cli.Command) error {
	if cmd.NArg() > 1 {
		return fmt.Errorf("too many arguments!")
	}
	source := cmd.Args().First()

	flagOutput := cmd.String("output")
	flagTemplate := cmd.String("template")
	flagList := cmd.Bool("list")
	flagListVars := cmd.Bool("list-vars")
	flagVarPairs := cmd.StringSlice("var")
	flagForce := cmd.Bool("force")
	flagVars, err := parseVars(flagVarPairs)
	if err != nil {
		return err
	}

	if flagList && flagListVars {
		return fmt.Errorf("--list and --list-vars cannot be used together!")
	}

	tmpl, coll, close, err := initResolve(ctx, source)
	if err != nil {
		return err
	}
	defer close()

	if tmpl == nil && coll != nil {
		if flagList {
			for _, t := range coll.Templates {
				fmt.Println(t.Config.Metadata.Slug)
			}
			return nil
		}

		if flagTemplate != "" {
			if selected := coll.Get(flagTemplate); selected != nil {
				tmpl = selected
			}
		} else if coll.Config.Templates.Default != "" {
			if selected := coll.Get(coll.Config.Templates.Default); selected != nil {
				tmpl = selected
			}
		}
	}

	if tmpl == nil {
		return fmt.Errorf("no template found!")
	}

	if flagListVars {
		for _, v := range tmpl.Config.Variables {
			fmt.Printf("%q: %q\n", v.Name, v.Default)
		}
		return nil
	}

	res, err := tmpl.Build(ctx, flagOutput, scaffold.BuildOptions{
		Variables: flagVars,
		Force:     flagForce,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Files: %v\n", res.FilesCreated)
	fmt.Printf("Dirs:  %v\n", res.DirsCreated)

	return nil
}

func initResolve(ctx context.Context, source string) (tmpl *scaffold.Template, coll *scaffold.Collection, close func(), err error) {
	switch source {
	case "":
		tmpl, coll, err = scaffold.LoadFS(ctx, embed.Scaffold, "scaffold")
	default:
		tmpl, coll, err = scaffold.Load(ctx, source)
	}

	if err != nil {
		return nil, nil, nil, err
	} else if tmpl == nil && coll == nil {
		return nil, nil, nil, fmt.Errorf("no templates found")
	} else if tmpl != nil {
		close = sync.OnceFunc(func() {
			tmpl.Close()
		})
	} else if coll != nil {
		close = sync.OnceFunc(func() {
			coll.Close()
		})
	}

	return tmpl, coll, close, nil
}

func parseVars(pairs []string) (map[string]any, error) {
	vars := make(map[string]any)

	for _, pair := range pairs {
		key, val, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --var %q (expected key=value)", pair)
		}
		vars[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}

	return vars, nil
}
