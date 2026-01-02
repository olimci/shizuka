package scaffold

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	TemplateFile   = "shizuka.template.toml"
	CollectionFile = "shizuka.collection.toml"
)

var gitKnownHosts = []string{
	"github.com/",
	"gitlab.com/",
	"bitbucket.org/",
	"codeberg.org/",
}

func Load(ctx context.Context, target string) (*Template, *Collection, error) {
	src, err := resolve(target)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving source: %w", err)
	}

	fsy, err := src.FS(ctx)
	if err != nil {
		src.Close()
		return nil, nil, fmt.Errorf("accessing source: %w", err)
	}

	root := src.Root()

	if _, err := fs.Stat(fsy, path.Join(root, CollectionFile)); err == nil {
		collection, err := LoadCollection(ctx, src, ".")
		if err != nil {
			src.Close()
			return nil, nil, err
		}
		return nil, collection, nil
	}

	if _, err := fs.Stat(fsy, path.Join(root, TemplateFile)); err == nil {
		template, err := LoadTemplate(ctx, src, ".")
		if err != nil {
			src.Close()
			return nil, nil, err
		}
		return template, nil, nil
	}

	src.Close()
	return nil, nil, fmt.Errorf("no %s or %s found at %s", TemplateFile, CollectionFile, target)
}

func LoadFS(ctx context.Context, fsy fs.FS, root string) (*Template, *Collection, error) {
	src := NewFSSource(fsy, root)

	if _, err := fs.Stat(fsy, path.Join(root, CollectionFile)); err == nil {
		collection, err := LoadCollection(ctx, src, ".")
		if err != nil {
			src.Close()
			return nil, nil, err
		}
		return nil, collection, nil
	}

	if _, err := fs.Stat(fsy, path.Join(root, TemplateFile)); err == nil {
		template, err := LoadTemplate(ctx, src, ".")
		if err != nil {
			src.Close()
			return nil, nil, err
		}
		return template, nil, nil
	}

	src.Close()
	return nil, nil, fmt.Errorf("no %s or %s found in %s", TemplateFile, CollectionFile, root)
}

func LoadTemplate(ctx context.Context, src Source, p string) (*Template, error) {
	fsy, err := src.FS(ctx)
	if err != nil {
		return nil, fmt.Errorf("accessing source: %w", err)
	}

	base := path.Join(src.Root(), p)

	file, err := fsy.Open(path.Join(base, TemplateFile))
	if err != nil {
		return nil, fmt.Errorf("opening template config file: %w", err)
	}
	defer file.Close()

	var config TemplateCfg
	if md, err := toml.NewDecoder(file).Decode(&config); err != nil {
		return nil, fmt.Errorf("decoding template config: %w", err)
	} else if len(md.Undecoded()) > 0 {
		return nil, fmt.Errorf("unknown keys in template config: %v", md.Undecoded())
	}

	return &Template{
		Config: config,
		source: src,
		Base:   base,
	}, nil
}

func LoadCollection(ctx context.Context, src Source, p string) (*Collection, error) {
	fsy, err := src.FS(ctx)
	if err != nil {
		return nil, fmt.Errorf("accessing source: %w", err)
	}

	base := path.Join(src.Root(), p)

	file, err := fsy.Open(path.Join(base, CollectionFile))
	if err != nil {
		return nil, fmt.Errorf("opening collection config file: %w", err)
	}
	defer file.Close()

	var config CollectionCfg
	if md, err := toml.NewDecoder(file).Decode(&config); err != nil {
		return nil, fmt.Errorf("decoding collection config: %w", err)
	} else if len(md.Undecoded()) > 0 {
		return nil, fmt.Errorf("unknown keys in collection config: %v", md.Undecoded())
	}

	templates := make([]*Template, len(config.Templates.Items))

	for i, slug := range config.Templates.Items {
		fullPath := path.Join(p, slug)
		template, err := LoadTemplate(ctx, src, fullPath)
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
func resolve(target string) (Source, error) {
	if isRemoteURL(target) {
		return NewRemoteSource(target), nil
	}

	if info, err := os.Stat(target); err == nil {
		if !info.IsDir() {
			return nil, fmt.Errorf("%s is not a directory", target)
		}
		return NewOSSource(target), nil
	}

	if looksLikeGitShorthand(target) {
		return NewRemoteSource("https://" + target), nil
	}

	return nil, fmt.Errorf("cannot resolve %s: path does not exist and is not a valid remote URL", target)
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
