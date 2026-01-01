package scaffold

type ScaffoldConfig struct {
	Metadata  ScaffoldConfigMeta           `toml:"metadata"`
	Files     ScaffoldConfigFiles          `toml:"files"`
	Variables map[string]ScaffoldConfigVar `toml:"variables"`
}

type ScaffoldConfigMeta struct {
	Name           string `toml:"name"`
	Description    string `toml:"description"`
	Version        string `toml:"version"`
	ShizukaVersion string `toml:"shizuka_version"`
}

type ScaffoldConfigFiles struct {
	StripSuffixes []string          `toml:"strip_suffixes"`
	Templates     []string          `toml:"templates"`
	Files         []string          `toml:"files"`
	Renames       map[string]string `toml:"renames"`
}

type ScaffoldConfigVar struct {
	Description string `toml:"description"`
	Default     string `toml:"default"`
}

type CollectionConfig struct {
	Metadata  CollectionConfigMeta      `toml:"metadata"`
	Scaffolds CollectionConfigScaffolds `toml:"scaffolds"`
}

type CollectionConfigScaffolds struct {
	Items   []string `toml:"items"`
	Default string   `toml:"default"`
}

type CollectionConfigMeta struct {
	Name           string `toml:"name"`
	Description    string `toml:"description"`
	Version        string `toml:"version"`
	ShizukaVersion string `toml:"shizuka_version"`
}
