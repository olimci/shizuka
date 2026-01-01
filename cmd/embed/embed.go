package embed

import "embed"

//go:embed templates/*
var Templates embed.FS

//go:embed all:scaffold
var Scaffold embed.FS
