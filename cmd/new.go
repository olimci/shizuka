package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/olimci/coffee"
	"github.com/olimci/shizuka/pkg/config"
	"github.com/urfave/cli/v3"
)

var newCmd = &cli.Command{
	Name:      "new",
	Usage:     "Create a new content file",
	ArgsUsage: "[path]",
	Flags:     newFlags(),
	Action:    newAction,
}

var xNewCmd = &cli.Command{
	Name:      "new",
	Usage:     "Create a new content file (non-interactive)",
	ArgsUsage: "[path]",
	Flags:     newFlags(),
	Action:    xNewAction,
}

func newFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   DefaultConfigPath,
			Usage:   "Config file path",
		},
		&cli.StringFlag{
			Name:  "title",
			Usage: "Page title",
		},
		&cli.StringFlag{
			Name:  "template",
			Usage: "Template name override",
		},
		&cli.StringFlag{
			Name:  "section",
			Usage: "Section override",
		},
		&cli.BoolFlag{
			Name:  "draft",
			Usage: "Create the page as a draft",
		},
		&cli.BoolFlag{
			Name:    "force",
			Aliases: []string{"f"},
			Usage:   "Overwrite an existing file",
		},
	}
}

type newRequest struct {
	ConfigPath string
	Path       string
	Title      string
	Template   string
	Section    string
	Draft      bool
	Force      bool
}

type newContext struct {
	ConfigDir       string
	ContentSource   string
	DefaultTemplate string
}

type newPageData struct {
	Slug     string
	Title    string
	Template string
	Section  string
	Date     string
	Draft    bool
}

type newResult struct {
	Path string
}

func newAction(ctx context.Context, cmd *cli.Command) error {
	req, err := newRequestFromCommand(cmd)
	if err != nil {
		return err
	}

	err = coffee.Do(func(ctx context.Context, c *coffee.Coffee) error {
		defer func() {
			_ = c.Clear()
		}()

		res, err := createNewContentInteractive(ctx, c, req)
		if err != nil {
			return err
		}

		_ = c.Logf("created %s", res.Path)
		return nil
	}, coffee.WithContext(ctx))
	if errors.Is(err, coffee.ErrNonInteractive) {
		return xNewAction(ctx, cmd)
	}
	return err
}

func xNewAction(_ context.Context, cmd *cli.Command) error {
	req, err := newRequestFromCommand(cmd)
	if err != nil {
		return err
	}

	res, err := createNewContent(req)
	if err != nil {
		return err
	}

	fmt.Printf("created %s\n", res.Path)
	return nil
}

func newRequestFromCommand(cmd *cli.Command) (newRequest, error) {
	if cmd.NArg() > 1 {
		return newRequest{}, fmt.Errorf("too many arguments")
	}

	return newRequest{
		ConfigPath: cmd.String("config"),
		Path:       strings.TrimSpace(cmd.Args().First()),
		Title:      strings.TrimSpace(cmd.String("title")),
		Template:   strings.TrimSpace(cmd.String("template")),
		Section:    strings.TrimSpace(cmd.String("section")),
		Draft:      cmd.Bool("draft"),
		Force:      cmd.Bool("force"),
	}, nil
}

func createNewContentInteractive(_ context.Context, c *coffee.Coffee, req newRequest) (*newResult, error) {
	nctx, err := loadNewContext(req.ConfigPath)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.Path) == "" {
		defaultPath := defaultNewPath(req.Title)

		_ = c.Logf("path (relative to %s):", nctx.ContentSource)
		relPath, err := c.AwaitInput(
			coffee.WithInputPlaceholder(defaultPath),
			coffee.WithInputValue(defaultPath),
		)
		if err != nil {
			return nil, err
		}
		req.Path = strings.TrimSpace(relPath)
	}

	if strings.TrimSpace(req.Title) == "" {
		defaultTitle := deriveTitle(req.Path)
		if defaultTitle == "" {
			defaultTitle = "New Page"
		}

		_ = c.Log("title:")
		title, err := c.AwaitInput(
			coffee.WithInputPlaceholder("Page title"),
			coffee.WithInputValue(defaultTitle),
		)
		if err != nil {
			return nil, err
		}
		req.Title = strings.TrimSpace(title)
	}

	return createNewContentWithContext(nctx, req)
}

func createNewContent(req newRequest) (*newResult, error) {
	nctx, err := loadNewContext(req.ConfigPath)
	if err != nil {
		return nil, err
	}

	return createNewContentWithContext(nctx, req)
}

func createNewContentWithContext(nctx *newContext, req newRequest) (*newResult, error) {
	relPath, err := normalizeNewPath(req.Path)
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = deriveTitle(relPath)
	}
	if title == "" {
		return nil, fmt.Errorf("title must not be empty")
	}

	isPost := looksLikePost(relPath, req.Section)
	slug := strings.TrimSuffix(relPath, path.Ext(relPath))

	pageData := newPageData{
		Slug:     slug,
		Title:    title,
		Template: defaultTemplateForNew(isPost, strings.TrimSpace(req.Template), nctx.DefaultTemplate),
		Section:  defaultSectionForNew(isPost, strings.TrimSpace(req.Section)),
		Date:     time.Now().Format("2006-01-02"),
		Draft:    req.Draft,
	}

	targetPath := filepath.Join(nctx.ConfigDir, filepath.FromSlash(nctx.ContentSource), filepath.FromSlash(relPath))
	if !req.Force {
		if _, err := os.Stat(targetPath); err == nil {
			return nil, fmt.Errorf(
				"file %s already exists (use force to overwrite)",
				filepath.ToSlash(filepath.Join(nctx.ContentSource, relPath)),
			)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating parent directory: %w", err)
	}

	if err := os.WriteFile(targetPath, []byte(builtInNewFileBody(pageData)), 0o644); err != nil {
		return nil, err
	}

	return &newResult{
		Path: filepath.ToSlash(filepath.Join(nctx.ContentSource, relPath)),
	}, nil
}

func loadNewContext(configPath string) (*newContext, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	configDir := filepath.Dir(configPath)
	if configDir == "" {
		configDir = "."
	}

	contentSource := "content"
	defaultTemplate := "page"
	if cfg.Build.Steps.Content != nil {
		if strings.TrimSpace(cfg.Build.Steps.Content.Source) != "" {
			contentSource = cfg.Build.Steps.Content.Source
		}
		if strings.TrimSpace(cfg.Build.Steps.Content.DefaultTemplate) != "" {
			defaultTemplate = cfg.Build.Steps.Content.DefaultTemplate
		}
	}

	return &newContext{
		ConfigDir:       configDir,
		ContentSource:   filepath.ToSlash(contentSource),
		DefaultTemplate: defaultTemplate,
	}, nil
}

func normalizeNewPath(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute paths are not supported: %q", rel)
	}

	rel = filepath.ToSlash(rel)
	rel = path.Clean(rel)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "." || rel == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	if strings.HasPrefix(rel, "../") || rel == ".." {
		return "", fmt.Errorf("path %q escapes content root", rel)
	}

	if path.Ext(rel) == "" {
		rel += ".md"
	}
	return rel, nil
}

func builtInNewFileBody(data newPageData) string {
	var b strings.Builder

	b.WriteString("---\n")
	b.WriteString("title: ")
	b.WriteString(strconv.Quote(data.Title))
	b.WriteByte('\n')

	if data.Template != "" {
		b.WriteString("template: ")
		b.WriteString(strconv.Quote(data.Template))
		b.WriteByte('\n')
	}
	if data.Section != "" {
		b.WriteString("sections: ")
		b.WriteString(strconv.Quote(data.Section))
		b.WriteByte('\n')
	}
	if data.Slug != "" {
		b.WriteString("slug: ")
		b.WriteString(strconv.Quote(data.Slug))
		b.WriteByte('\n')
	}
	if data.Date != "" {
		b.WriteString("date: ")
		b.WriteString(data.Date)
		b.WriteByte('\n')
	}
	if data.Draft {
		b.WriteString("draft: true\n")
	}

	b.WriteString("---\n\n")
	return b.String()
}

func defaultTemplateForNew(isPost bool, explicit, configDefault string) string {
	if explicit != "" {
		return explicit
	}
	if isPost {
		return "post"
	}
	if configDefault != "" {
		return configDefault
	}
	return "page"
}

func defaultSectionForNew(isPost bool, explicit string) string {
	if explicit != "" {
		return explicit
	}
	if isPost {
		return "posts"
	}
	return ""
}

func looksLikePost(relPath, section string) bool {
	if strings.TrimSpace(section) == "posts" {
		return true
	}

	relPath = filepath.ToSlash(strings.TrimSpace(relPath))
	return strings.HasPrefix(relPath, "posts/")
}

func defaultNewPath(title string) string {
	stem := slugify(title)
	if stem == "" {
		return "about.md"
	}
	return stem + ".md"
}

func deriveTitle(relPath string) string {
	stem := strings.TrimSuffix(path.Base(strings.TrimSpace(relPath)), path.Ext(strings.TrimSpace(relPath)))
	if stem == "" {
		return ""
	}

	parts := strings.FieldsFunc(stem, func(r rune) bool {
		return r == '-' || r == '_' || unicode.IsSpace(r)
	})
	for i, part := range parts {
		parts[i] = titleWord(part)
	}

	return strings.Join(parts, " ")
}

func titleWord(word string) string {
	if word == "" {
		return ""
	}

	runes := []rune(strings.ToLower(word))
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func slugify(s string) string {
	var b strings.Builder
	lastDash := false

	for _, r := range strings.TrimSpace(strings.ToLower(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if b.Len() == 0 || lastDash {
				continue
			}
			b.WriteByte('-')
			lastDash = true
		}
	}

	return strings.Trim(b.String(), "-")
}
