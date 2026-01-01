package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/olimci/shizuka/cmd/embed"
	"github.com/olimci/shizuka/pkg/scaffold"
	"github.com/olimci/shizuka/pkg/version"
	"github.com/urfave/cli/v3"
)

func Init(ctx context.Context, cmd *cli.Command) error {
	source := cmd.String("source")
	templateName := cmd.String("template")
	force := cmd.Bool("force")
	quiet := cmd.Bool("quiet")

	if cmd.Bool("list") {
		return listTemplates(ctx, source)
	}

	targetDir := "."
	if cmd.NArg() > 0 {
		targetDir = cmd.Args().First()
	}

	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("resolving target directory: %w", err)
	}

	siteName := cmd.String("name")
	if siteName == "" {
		siteName = deriveSiteName(absTargetDir)
	}

	vars := map[string]any{
		"SiteName": siteName,
		"SiteSlug": toSlug(siteName),
		"Version":  version.String(),
		"Year":     time.Now().Format("2006"),
	}

	scaf, err := loadScaffold(ctx, source, templateName)
	if err != nil {
		return err
	}
	defer scaf.Close()

	if !quiet {
		if targetDir == "." {
			fmt.Println("Creating new Shizuka site in current directory...")
		} else {
			fmt.Printf("Creating new Shizuka site in %s...\n", targetDir)
		}
		fmt.Println()
	}

	result, err := scaf.Build(
		ctx, absTargetDir,
		scaffold.WithVariables(vars),
		scaffold.WithForce(force),
	)
	if err != nil {
		return err
	}

	if !quiet {
		for _, f := range result.FilesCreated {
			fmt.Printf("  ✓ %s\n", f)
		}
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

// loadScaffold loads a scaffold from the given source.
// If source is empty, uses embedded templates.
// If source points to a collection, templateName selects which scaffold to use.
func loadScaffold(ctx context.Context, source, templateName string) (*scaffold.Scaffold, error) {
	if source == "" {
		return loadEmbeddedScaffold(ctx, templateName)
	}

	scaf, collection, err := scaffold.Load(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("loading source: %w", err)
	}

	if scaf != nil {
		return scaf, nil
	}

	if templateName == "" {
		templateName = collection.Config.Scaffolds.Default
	}

	if templateName == "" {
		var names []string
		for _, s := range collection.Scaffolds {
			names = append(names, s.Config.Metadata.Name)
		}
		collection.Close()
		return nil, fmt.Errorf("source is a collection, please specify --template. Available: %s", strings.Join(names, ", "))
	}

	for _, s := range collection.Scaffolds {
		if s.Config.Metadata.Name == templateName {
			return s, nil
		}
	}

	var names []string
	for _, s := range collection.Scaffolds {
		names = append(names, s.Config.Metadata.Name)
	}
	collection.Close()
	return nil, fmt.Errorf("template %q not found in collection. Available: %s", templateName, strings.Join(names, ", "))
}

// loadEmbeddedScaffold loads a scaffold from the embedded templates
func loadEmbeddedScaffold(ctx context.Context, templateName string) (*scaffold.Scaffold, error) {
	src := scaffold.NewFSSource(embed.Scaffold, "scaffold")

	collection, err := scaffold.LoadCollection(ctx, src, ".")
	if err == nil {
		if templateName == "" {
			templateName = collection.Config.Scaffolds.Default
		}

		if templateName == "" {
			var names []string
			for _, s := range collection.Scaffolds {
				names = append(names, s.Config.Metadata.Name)
			}
			collection.Close()
			return nil, fmt.Errorf("no template specified and no default set. Available: %s", strings.Join(names, ", "))
		}

		for _, s := range collection.Scaffolds {
			if s.Config.Metadata.Name == templateName {
				return s, nil
			}
		}

		var names []string
		for _, s := range collection.Scaffolds {
			names = append(names, s.Config.Metadata.Name)
		}
		collection.Close()
		return nil, fmt.Errorf("template %q not found. Available: %s", templateName, strings.Join(names, ", "))
	}

	if templateName == "" {
		return nil, fmt.Errorf("no embedded collection found and no template specified")
	}

	scaf, err := scaffold.LoadScaffold(ctx, src, templateName)
	if err != nil {
		return nil, fmt.Errorf("loading embedded template %q: %w", templateName, err)
	}

	return scaf, nil
}

func listTemplates(ctx context.Context, source string) error {
	if source == "" {
		return listEmbeddedTemplates(ctx)
	}

	scaf, collection, err := scaffold.Load(ctx, source)
	if err != nil {
		return fmt.Errorf("loading source: %w", err)
	}

	if scaf != nil {
		defer scaf.Close()
		fmt.Println("Source contains a single template:")
		fmt.Println()
		fmt.Printf("  %-12s  %s\n", scaf.Config.Metadata.Name, scaf.Config.Metadata.Description)
		return nil
	}

	defer collection.Close()
	fmt.Printf("Available templates from %s:\n", source)
	fmt.Println()

	for _, s := range collection.Scaffolds {
		fmt.Printf("  %-12s  %s\n", s.Config.Metadata.Name, s.Config.Metadata.Description)
	}

	fmt.Println()
	fmt.Println("Use: shizuka init --source <source> --template <name>")

	return nil
}

func listEmbeddedTemplates(ctx context.Context) error {
	src := scaffold.NewFSSource(embed.Scaffold, "scaffold")

	collection, err := scaffold.LoadCollection(ctx, src, ".")
	if err == nil {
		defer collection.Close()
		fmt.Println("Available templates:")
		fmt.Println()

		for _, s := range collection.Scaffolds {
			fmt.Printf("  %-12s  %s\n", s.Config.Metadata.Name, s.Config.Metadata.Description)
		}

		fmt.Println()
		fmt.Println("Use: shizuka init --template <name>")
		return nil
	}

	fmt.Println("No embedded templates found.")
	fmt.Println()
	fmt.Println("Use --source to specify a local path or remote URL:")
	fmt.Println("  shizuka init --source ./my-templates")
	fmt.Println("  shizuka init --source github.com/user/templates")

	return nil
}

// deriveSiteName generates a site name from the directory path
func deriveSiteName(dir string) string {
	if dir == "" || dir == "." {
		if cwd, err := os.Getwd(); err == nil {
			dir = cwd
		} else {
			return "My Site"
		}
	}

	name := filepath.Base(dir)
	if name == "." || name == "/" {
		return "My Site"
	}

	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	return toTitleCase(name)
}

var (
	nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)
	dashRuns     = regexp.MustCompile(`-+`)
)

// toSlug converts a string to a URL-friendly slug
func toSlug(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", " ")
	s = nonSlugChars.ReplaceAllString(s, "-")
	s = dashRuns.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// toTitleCase converts a string to title case
func toTitleCase(s string) string {
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			runes := []rune(word)
			runes[0] = unicode.ToUpper(runes[0])
			words[i] = string(runes)
		}
	}
	return strings.Join(words, " ")
}
