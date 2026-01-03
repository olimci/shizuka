package embed

import "embed"

//go:embed all:scaffold
var Scaffold embed.FS

//go:embed all:templates
var Templates embed.FS
