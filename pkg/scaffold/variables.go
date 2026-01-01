package scaffold

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var (
	nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)
	dashRuns     = regexp.MustCompile(`-+`)
)

type Variables struct {
	SiteName string
	SiteSlug string
	Version  string
	Year     string
	Author   string
	Custom   map[string]string
}

type VariablesConfig struct {
	Directory string
	SiteName  string
	Version   string
	Author    string
	Custom    map[string]string
}

func NewVariables(cfg VariablesConfig) *Variables {
	siteName := cfg.SiteName
	if siteName == "" {
		siteName = deriveSiteName(cfg.Directory)
	}

	vars := &Variables{
		SiteName: siteName,
		SiteSlug: toSlug(siteName),
		Version:  cfg.Version,
		Year:     time.Now().Format("2006"),
		Author:   cfg.Author,
		Custom:   cfg.Custom,
	}

	if vars.Custom == nil {
		vars.Custom = make(map[string]string)
	}

	return vars
}

func (v *Variables) ToMap() map[string]any {
	m := map[string]any{
		"SiteName": v.SiteName,
		"SiteSlug": v.SiteSlug,
		"Version":  v.Version,
		"Year":     v.Year,
		"Author":   v.Author,
	}

	for k, val := range v.Custom {
		m[k] = val
	}

	return m
}

func deriveSiteName(dir string) string {
	if dir == "" || dir == "." {
		return "My Site"
	}

	name := filepath.Base(dir)
	if name == "." || name == "/" {
		return "My Site"
	}

	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	return toTitleCase(name)
}

func toSlug(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", " ")
	s = nonSlugChars.ReplaceAllString(s, "-")
	s = dashRuns.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

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
