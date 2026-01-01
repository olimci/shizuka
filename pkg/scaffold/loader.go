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
	ScaffoldPath   = "shizuka.scaffold.toml"
	CollectionPath = "shizuka.collection.toml"
)

var gitKnownHosts = []string{
	"github.com/",
	"gitlab.com/",
	"bitbucket.org/",
	"codeberg.org/",
}

func Load(ctx context.Context, target string) (*Scaffold, *Collection, error) {
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

	if _, err := fs.Stat(fsy, path.Join(root, CollectionPath)); err == nil {
		collection, err := LoadCollection(ctx, src, ".")
		if err != nil {
			src.Close()
			return nil, nil, err
		}
		return nil, collection, nil
	}

	if _, err := fs.Stat(fsy, path.Join(root, ScaffoldPath)); err == nil {
		scaffold, err := LoadScaffold(ctx, src, ".")
		if err != nil {
			src.Close()
			return nil, nil, err
		}
		return scaffold, nil, nil
	}

	src.Close()
	return nil, nil, fmt.Errorf("no %s or %s found at %s", ScaffoldPath, CollectionPath, target)
}

func LoadScaffold(ctx context.Context, src source, p string) (*Scaffold, error) {
	fsy, err := src.FS(ctx)
	if err != nil {
		return nil, fmt.Errorf("accessing source: %w", err)
	}

	base := path.Join(src.Root(), p)

	file, err := fsy.Open(path.Join(base, ScaffoldPath))
	if err != nil {
		return nil, fmt.Errorf("opening scaffold config file: %w", err)
	}
	defer file.Close()

	var config ScaffoldConfig
	if md, err := toml.NewDecoder(file).Decode(&config); err != nil {
		return nil, fmt.Errorf("decoding scaffold config: %w", err)
	} else if len(md.Undecoded()) > 0 {
		return nil, fmt.Errorf("unknown keys in scaffold config: %v", md.Undecoded())
	}

	return &Scaffold{
		Config: config,
		source: src,
		Base:   base,
	}, nil
}

func LoadCollection(ctx context.Context, src source, p string) (*Collection, error) {
	fsy, err := src.FS(ctx)
	if err != nil {
		return nil, fmt.Errorf("accessing source: %w", err)
	}

	base := path.Join(src.Root(), p)

	file, err := fsy.Open(path.Join(base, CollectionPath))
	if err != nil {
		return nil, fmt.Errorf("opening collection config file: %w", err)
	}
	defer file.Close()

	var config CollectionConfig
	if md, err := toml.NewDecoder(file).Decode(&config); err != nil {
		return nil, fmt.Errorf("decoding collection config: %w", err)
	} else if len(md.Undecoded()) > 0 {
		return nil, fmt.Errorf("unknown keys in collection config: %v", md.Undecoded())
	}

	scaffolds := make([]*Scaffold, len(config.Scaffolds.Items))

	for i, scaffoldPath := range config.Scaffolds.Items {
		fullPath := path.Join(p, scaffoldPath)
		scaffold, err := LoadScaffold(ctx, src, fullPath)
		if err != nil {
			return nil, fmt.Errorf("loading scaffold %s: %w", scaffoldPath, err)
		}
		scaffolds[i] = scaffold
	}

	return &Collection{
		Config:    config,
		source:    src,
		Base:      base,
		Scaffolds: scaffolds,
	}, nil
}

// resolve determines the source type from the target string and returns the appropriate source
func resolve(target string) (source, error) {
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
