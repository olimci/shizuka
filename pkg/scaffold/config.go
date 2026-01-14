package scaffold

type TemplateCfg struct {
	Metadata  TemplateCfgMeta           `toml:"metadata" yaml:"metadata" json:"metadata"`
	Files     TemplateCfgFiles          `toml:"files" yaml:"files" json:"files"`
	Variables map[string]TemplateCfgVar `toml:"variables" yaml:"variables" json:"variables"`
}

type TemplateCfgMeta struct {
	Name           string `toml:"name" yaml:"name" json:"name"`
	Slug           string `toml:"slug" yaml:"slug" json:"slug"`
	Description    string `toml:"description" yaml:"description" json:"description"`
	Version        string `toml:"version" yaml:"version" json:"version"`
	ShizukaVersion string `toml:"shizuka_version" yaml:"shizuka_version" json:"shizuka_version"`
}

type TemplateCfgFiles struct {
	StripSuffixes []string          `toml:"strip_suffixes" yaml:"strip_suffixes" json:"strip_suffixes"`
	Templates     []string          `toml:"templates" yaml:"templates" json:"templates"`
	Files         []string          `toml:"files" yaml:"files" json:"files"`
	Renames       map[string]string `toml:"renames" yaml:"renames" json:"renames"`
}

type TemplateCfgVar struct {
	Name        string `toml:"name" yaml:"name" json:"name"`
	Description string `toml:"description" yaml:"description" json:"description"`
	Default     string `toml:"default" yaml:"default" json:"default"`
}

type CollectionCfg struct {
	Metadata  CollectionCfgMeta      `toml:"metadata" yaml:"metadata" json:"metadata"`
	Templates CollectionCfgTemplates `toml:"templates" yaml:"templates" json:"templates"`
}

type CollectionCfgTemplates struct {
	Items   []string `toml:"items" yaml:"items" json:"items"`
	Default string   `toml:"default" yaml:"default" json:"default"`
}

type CollectionCfgMeta struct {
	Name           string `toml:"name" yaml:"name" json:"name"`
	Description    string `toml:"description" yaml:"description" json:"description"`
	Version        string `toml:"version" yaml:"version" json:"version"`
	ShizukaVersion string `toml:"shizuka_version" yaml:"shizuka_version" json:"shizuka_version"`
}
