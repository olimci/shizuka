// Package embed provides embedded files for the shizuka CLI.
package embed

import "embed"

//go:embed templates/*
var Templates embed.FS
