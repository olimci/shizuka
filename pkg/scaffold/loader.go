package scaffold

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"strings"
)

const (
	TemplateFileBase   = "shizuka.template"
	CollectionFileBase = "shizuka.collection"
)

var gitKnownHosts = []string{
	"github.com/",
	"gitlab.com/",
	"bitbucket.org/",
	"codeberg.org/",
}

var (
	ErrFailedToLoad = fmt.Errorf("failed to load scaffold")
)

var (
	preferredConfigExts = []string{".toml", ".yaml", ".yml", ".json"}
)

func configCandidates(base string) []string {
	out := make([]string, 0, len(preferredConfigExts))
	for _, ext := range preferredConfigExts {
		out = append(out, base+ext)
	}
	return out
}

func findConfigFile(fsy fs.FS, base string) (string, bool, error) {
	for _, candidate := range configCandidates(base) {
		_, err := fs.Stat(fsy, candidate)
		if err == nil {
			return candidate, true, nil
		}
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		return "", false, err
	}
	return "", false, nil
}

func Load(ctx context.Context, target string) (*Template, *Collection, error) {
	src, err := resolve(target)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: resolving source: %w", ErrFailedToLoad, err)
	}

	if err := src.init(ctx); err != nil {
		src.Close()
		return nil, nil, fmt.Errorf("%w: accessing source: %w", ErrFailedToLoad, err)
	}

	fsy := src.fsys
	root := src.root

	if _, ok, err := findConfigFile(fsy, path.Join(root, CollectionFileBase)); err != nil {
		return nil, nil, fmt.Errorf("%w: checking collection config: %w", ErrFailedToLoad, err)
	} else if ok {
		collection, err := LoadCollection(src, ".")
		if err != nil {
			src.Close()
			return nil, nil, err
		}
		return nil, collection, nil
	}

	if _, ok, err := findConfigFile(fsy, path.Join(root, TemplateFileBase)); err != nil {
		return nil, nil, fmt.Errorf("%w: checking template config: %w", ErrFailedToLoad, err)
	} else if ok {
		template, err := LoadTemplate(src, ".")
		if err != nil {
			src.Close()
			return nil, nil, err
		}
		return template, nil, nil
	}

	return nil, nil, fmt.Errorf("%w: no %v or %v found at %s", ErrFailedToLoad, configCandidates(TemplateFileBase), configCandidates(CollectionFileBase), target)
}

func LoadFS(_ context.Context, fsy fs.FS, root string) (*Template, *Collection, error) {
	src := newSource(fsy, root, nil)

	if _, ok, err := findConfigFile(fsy, path.Join(root, CollectionFileBase)); err != nil {
		src.Close()
		return nil, nil, fmt.Errorf("%w: checking collection config: %w", ErrFailedToLoad, err)
	} else if ok {
		collection, err := LoadCollection(src, ".")
		if err != nil {
			src.Close()
			return nil, nil, fmt.Errorf("%w: %w", ErrFailedToLoad, err)
		}
		return nil, collection, nil
	}

	if _, ok, err := findConfigFile(fsy, path.Join(root, TemplateFileBase)); err != nil {
		src.Close()
		return nil, nil, fmt.Errorf("%w: checking template config: %w", ErrFailedToLoad, err)
	} else if ok {
		template, err := LoadTemplate(src, ".")
		if err != nil {
			src.Close()
			return nil, nil, fmt.Errorf("%w: %w", ErrFailedToLoad, err)
		}
		return template, nil, nil
	}

	return nil, nil, fmt.Errorf("%w: no %v or %v found in %s", ErrFailedToLoad, configCandidates(TemplateFileBase), configCandidates(CollectionFileBase), root)
}

func LoadTemplate(src *source, p string) (*Template, error) {
	base := path.Join(src.root, p)

	cfgPath, ok, err := findConfigFile(src.fsys, path.Join(base, TemplateFileBase))
	if err != nil {
		return nil, fmt.Errorf("finding template config file: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("no template config found (expected one of %v)", configCandidates(TemplateFileBase))
	}

	file, err := src.fsys.Open(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("opening template config file: %w", err)
	}
	defer file.Close()

	var config TemplateCfg
	if err := decodeConfigFile(cfgPath, file, &config); err != nil {
		return nil, fmt.Errorf("decoding template config: %w", err)
	}

	return &Template{
		Config: config,
		source: src,
		Base:   base,
	}, nil
}

func LoadCollection(src *source, p string) (*Collection, error) {
	base := path.Join(src.root, p)

	cfgPath, ok, err := findConfigFile(src.fsys, path.Join(base, CollectionFileBase))
	if err != nil {
		return nil, fmt.Errorf("finding collection config file: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("no collection config found (expected one of %v)", configCandidates(CollectionFileBase))
	}

	file, err := src.fsys.Open(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("opening collection config file: %w", err)
	}
	defer file.Close()

	var config CollectionCfg
	if err := decodeConfigFile(cfgPath, file, &config); err != nil {
		return nil, fmt.Errorf("decoding collection config: %w", err)
	}

	templates := make([]*Template, len(config.Templates.Items))

	for i, slug := range config.Templates.Items {
		fullPath := path.Join(p, slug)
		template, err := LoadTemplate(src, fullPath)
		if err != nil {
			return nil, fmt.Errorf("loading template %s: %w", slug, err)
		}

		if template.Config.Metadata.Slug != slug {
			return nil, fmt.Errorf("template slug %s does not match directory name %s", template.Config.Metadata.Slug, slug)
		}

		templates[i] = template
	}

	return &Collection{
		Config:    config,
		source:    src,
		Base:      base,
		Templates: templates,
	}, nil
}

// resolve determines the source type from the target string and returns the appropriate source
func resolve(target string) (*source, error) {
	if isRemoteURL(target) {
		return newRemoteSource(target), nil
	}

	if info, err := os.Stat(target); err == nil {
		if !info.IsDir() {
			return nil, fmt.Errorf("%s is not a directory", target)
		}
		return newSource(os.DirFS(target), ".", nil), nil
	}

	if looksLikeGitShorthand(target) {
		return newRemoteSource("https://" + target), nil
	}

	return nil, fmt.Errorf("cannot resolve %s: path does not exist and is not a valid remote URL", target)
}

type source struct {
	fsys    fs.FS
	root    string
	close   func() error
	prepare func(context.Context) (fs.FS, error)
}

func newSource(fsys fs.FS, root string, close func() error) *source {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	if close == nil {
		close = func() error { return nil }
	}
	return &source{
		fsys:  fsys,
		root:  root,
		close: close,
	}
}

func newRemoteSource(url string) *source {
	var tempDir string
	return &source{
		root: ".",
		close: func() error {
			if tempDir == "" {
				return nil
			}
			return os.RemoveAll(tempDir)
		},
		prepare: func(ctx context.Context) (fs.FS, error) {
			if _, err := exec.LookPath("git"); err != nil {
				return nil, fmt.Errorf("git is required for remote sources: %w", err)
			}

			dir, err := os.MkdirTemp("", "shizuka-source-*")
			if err != nil {
				return nil, fmt.Errorf("creating temp directory: %w", err)
			}

			cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", url, dir)
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				_ = os.RemoveAll(dir)
				return nil, fmt.Errorf("cloning repository: %w", err)
			}

			tempDir = dir
			return os.DirFS(dir), nil
		},
	}
}

func (s *source) init(ctx context.Context) error {
	if s.fsys != nil {
		return nil
	}
	if s.prepare == nil {
		return fmt.Errorf("source is not initialized")
	}

	fsys, err := s.prepare(ctx)
	if err != nil {
		return err
	}

	s.fsys = fsys
	s.prepare = nil
	return nil
}

func (s *source) Close() error {
	if s == nil || s.close == nil {
		return nil
	}
	return s.close()
}

func isRemoteURL(target string) bool {
	return strings.HasPrefix(target, "https://") ||
		strings.HasPrefix(target, "http://") ||
		strings.HasPrefix(target, "git://") ||
		strings.HasPrefix(target, "git@")
}

func looksLikeGitShorthand(target string) bool {
	for _, host := range gitKnownHosts {
		if strings.HasPrefix(target, host) {
			return true
		}
	}

	return false
}
