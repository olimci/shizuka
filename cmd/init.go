package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/olimci/shizuka/cmd/embed"
	"github.com/olimci/shizuka/pkg/scaffold"
	"github.com/olimci/shizuka/pkg/version"
	"github.com/urfave/cli/v3"
)

func Init(ctx context.Context, cmd *cli.Command) error {
	if cmd.Bool("list-templates") {
		return listTemplates()
	}

	targetDir := "."
	if cmd.NArg() > 0 {
		targetDir = cmd.Args().First()
	}

	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("resolving target directory: %w", err)
	}

	templateName := cmd.String("template")
	force := cmd.Bool("force")
	quiet := cmd.Bool("quiet")

	scaffolder, err := scaffold.NewScaffolderWithEmbedded(
		embed.Scaffold,
		"scaffold",
		scaffold.WithOutput(os.Stdout),
		scaffold.WithQuiet(quiet),
	)
	if err != nil {
		return fmt.Errorf("initializing scaffolder: %w", err)
	}

	vars := scaffold.NewVariables(scaffold.VariablesConfig{
		Directory: absTargetDir,
		SiteName:  cmd.String("name"),
		Version:   version.String(),
	})

	if !quiet {
		if targetDir == "." {
			fmt.Println("Creating new Shizuka site in current directory...")
		} else {
			fmt.Printf("Creating new Shizuka site in %s...\n", targetDir)
		}
		fmt.Println()
	}

	result, err := scaffolder.Scaffold(templateName, absTargetDir, vars, force)
	if err != nil {
		return err
	}

	if !quiet {
		fmt.Println()
		fmt.Printf("Done! Created %d files.\n", len(result.FilesCreated))
		fmt.Println()
		fmt.Println("Next steps:")
		if targetDir != "." {
			fmt.Printf("  cd %s\n", targetDir)
		}
		fmt.Println("  shizuka dev     # Start development server")
		fmt.Println("  shizuka build   # Build for production")
	}

	return nil
}

func listTemplates() error {
	scaffolder, err := scaffold.NewScaffolderWithEmbedded(
		embed.Scaffold,
		"scaffold",
	)
	if err != nil {
		return fmt.Errorf("initializing scaffolder: %w", err)
	}

	templates := scaffolder.ListTemplates()

	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Name < templates[j].Name
	})

	fmt.Println("Available templates:")
	fmt.Println()

	for _, t := range templates {
		fmt.Printf("  %-12s  %s\n", t.Name, t.Description)
	}

	fmt.Println()
	fmt.Println("Use: shizuka init --template <name>")

	return nil
}
