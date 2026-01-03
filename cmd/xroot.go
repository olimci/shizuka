package cmd

import (
	"github.com/urfave/cli/v3"
)

// xCmd returns the non-interactive subcommand group
func xCmd() *cli.Command {
	return &cli.Command{
		Name:  "x",
		Usage: "Non-interactive commands (for scripts and CI)",
		Commands: []*cli.Command{
			xInitCmd(),
			xBuildCmd(),
			xDevCmd(),
		},
	}
}

func xInitCmd() *cli.Command {
	return &cli.Command{
		Name:      "init",
		Usage:     "Scaffold a new Shizuka site (non-interactive)",
		ArgsUsage: "[source] [directory]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "source",
				Aliases: []string{"s"},
				Usage:   "Template source (local path or remote URL; overrides positional source)",
			},
			&cli.StringFlag{
				Name:    "template",
				Aliases: []string{"t"},
				Usage:   "Template name (for collections)",
			},
			&cli.StringSliceFlag{
				Name:  "var",
				Usage: "Template variable (key=value, repeatable)",
			},
			&cli.StringFlag{
				Name:  "vars-file",
				Usage: "Variables file (.toml, .yaml, .yml, .json)",
			},
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Overwrite existing files",
			},
			&cli.BoolFlag{
				Name:    "list",
				Aliases: []string{"l"},
				Usage:   "List available templates",
			},
			&cli.BoolFlag{
				Name:  "list-vars",
				Usage: "List template variables",
			},
		},
		Action: runXInit,
	}
}

func xBuildCmd() *cli.Command {
	return &cli.Command{
		Name:   "build",
		Usage:  "Build the site (non-interactive)",
		Flags:  buildFlags(),
		Action: runXBuild,
	}
}

func xDevCmd() *cli.Command {
	return &cli.Command{
		Name:   "dev",
		Usage:  "Start development server (non-interactive, logs to stdout)",
		Flags:  devFlags(),
		Action: runXDev,
	}
}
