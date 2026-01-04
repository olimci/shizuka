package build

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/olimci/shizuka/pkg/manifest"
)

// static creates a new static artefact
func makeStatic(owner, source, target string) manifest.Artefact {
	return manifest.Artefact{
		Claim: manifest.Claim{
			Owner:  owner,
			Source: source,
			Target: target,
		},
		Builder: func(w io.Writer) error {
			file, err := os.Open(source)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(w, file)
			return err
		},
	}
}

// makeTarget creates a new target path based on the root and relative path
func makeTarget(root, rel string) (src, dst string, err error) {
	dir, base := filepath.Split(rel)

	name := strings.TrimSuffix(base, filepath.Ext(base))

	src = filepath.Join(root, rel)

	if name == "index" {
		return src, filepath.Join(dir, "index.html"), nil
	} else {
		return src, filepath.Join(dir, name, "index.html"), nil
	}
}

func makeTextArtefact(owner, target, content string) manifest.Artefact {
	return manifest.Artefact{
		Claim: manifest.Claim{
			Owner:  owner,
			Source: "config",
			Target: target,
		},
		Builder: func(w io.Writer) error {
			_, err := io.WriteString(w, content)
			return err
		},
	}
}

func renderHeaders(headers map[string]map[string]string) string {
	paths := make([]string, 0, len(headers))
	for path := range headers {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var b strings.Builder
	for i, path := range paths {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(path)
		b.WriteString("\n")

		entries := headers[path]
		keys := make([]string, 0, len(entries))
		for key := range entries {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			b.WriteString("  ")
			b.WriteString(key)
			b.WriteString(": ")
			b.WriteString(entries[key])
			b.WriteString("\n")
		}
	}

	return b.String()
}

func renderRedirects(redirects []Redirect) string {
	sorted := slices.Clone(redirects)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].From == sorted[j].From {
			if sorted[i].To == sorted[j].To {
				return sorted[i].Status < sorted[j].Status
			}
			return sorted[i].To < sorted[j].To
		}
		return sorted[i].From < sorted[j].From
	})

	var b strings.Builder
	for _, redirect := range sorted {
		b.WriteString(redirect.From)
		b.WriteString(" ")
		b.WriteString(redirect.To)
		if redirect.Status != 0 {
			b.WriteString(" ")
			b.WriteString(fmt.Sprintf("%d", redirect.Status))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func targetToPath(target string) string {
	target = filepath.ToSlash(strings.TrimSpace(target))
	if target == "" {
		return "/"
	}
	if target == "index.html" {
		return "/"
	}
	if strings.HasSuffix(target, "/index.html") {
		target = strings.TrimSuffix(target, "/index.html")
	}
	target = strings.TrimSuffix(target, ".html")
	target = strings.Trim(target, "/")
	if target == "" {
		return "/"
	}
	return "/" + target
}

func shortenPath(root, slug string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "/s"
	}
	if !strings.HasPrefix(root, "/") {
		root = "/" + root
	}
	root = strings.TrimSuffix(root, "/")

	slug = strings.TrimSpace(slug)
	slug = strings.TrimPrefix(slug, "/")
	if slug == "" {
		return root
	}
	return root + "/" + slug
}
