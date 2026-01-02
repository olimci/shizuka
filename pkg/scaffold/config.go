package scaffold

type TemplateCfg struct {
	Metadata  TemplateCfgMeta           `toml:"metadata"`
	Files     TemplateCfgFiles          `toml:"files"`
	Variables map[string]TemplateCfgVar `toml:"variables"`
}

type TemplateCfgMeta struct {
	Name           string `toml:"name"`
	Slug           string `toml:"slug"`
	Description    string `toml:"description"`
	Version        string `toml:"version"`
	ShizukaVersion string `toml:"shizuka_version"`
}

type TemplateCfgFiles struct {
	StripSuffixes []string          `toml:"strip_suffixes"`
	Templates     []string          `toml:"templates"`
	Files         []string          `toml:"files"`
	Renames       map[string]string `toml:"renames"`
}

type TemplateCfgVar struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Default     string `toml:"default"`
}

type CollectionCfg struct {
	Metadata  CollectionCfgMeta      `toml:"metadata"`
	Templates CollectionCfgTemplates `toml:"templates"`
}

type CollectionCfgTemplates struct {
	Items   []string `toml:"items"`
	Default string   `toml:"default"`
}

type CollectionCfgMeta struct {
	Name           string `toml:"name"`
	Description    string `toml:"description"`
	Version        string `toml:"version"`
	ShizukaVersion string `toml:"shizuka_version"`
}
