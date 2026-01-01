package build

import (
	"bytes"
	"context"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStepContent_FallbackTemplate(t *testing.T) {
	// Create a temporary directory for our test
	tmpDir, err := os.MkdirTemp("", "shizuka-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create directory structure
	contentDir := filepath.Join(tmpDir, "content")
	templatesDir := filepath.Join(tmpDir, "templates")
	outputDir := filepath.Join(tmpDir, "dist")

	for _, dir := range []string{contentDir, templatesDir, outputDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create a content file with a missing template
	contentFile := filepath.Join(contentDir, "test.md")
	content := `---
title: "Test Page"
description: "A test page"
date: 2024-01-01
template: "nonexistent"
---

# Hello World

This is test content.
`
	if err := os.WriteFile(contentFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write content file: %v", err)
	}

	// Create a basic template (but not the one referenced by the content)
	templateFile := filepath.Join(templatesDir, "page.html")
	templateContent := `<!DOCTYPE html><html><body>{{ .Page.Body }}</body></html>`
	if err := os.WriteFile(templateFile, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template file: %v", err)
	}

	// Create a fallback template
	fallbackTmpl, err := template.New("fallback").Parse(`<!DOCTYPE html>
<html>
<head><title>{{ .Page.Title }} - Fallback</title></head>
<body>
<h1>FALLBACK: {{ .Page.Title }}</h1>
<p>Source: {{ .Meta.Source }}</p>
<p>Target: {{ .Meta.Target }}</p>
<p>Template: {{ .Meta.Template }}</p>
<p>Dev: {{ .SiteMeta.Dev }}</p>
<div>{{ .Page.Body }}</div>
</body>
</html>`)
	if err != nil {
		t.Fatalf("failed to parse fallback template: %v", err)
	}

	// Create config
	config := &Config{
		Site: SiteConfig{
			Title: "Test Site",
		},
		Build: BuildConfig{
			ContentDir:    contentDir,
			StaticDir:     filepath.Join(tmpDir, "static"),
			TemplatesGlob: filepath.Join(templatesDir, "*.html"),
			OutputDir:     outputDir,
		},
	}

	// Create a static dir (even if empty)
	os.MkdirAll(config.Build.StaticDir, 0755)

	// Create diagnostic collector
	collector := NewDiagnosticCollector()

	// Run build with fallback template and lenient errors (simulating dev mode)
	steps := []Step{
		StepContent(),
	}

	opts := []Option{
		WithContext(context.Background()),
		WithDiagnosticSink(collector),
		WithFallbackTemplate(fallbackTmpl),
		WithLenientErrors(),
		WithDev(),
	}

	err = Build(steps, config, opts...)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// Check that a warning was reported (not an error)
	diags := collector.Diagnostics()
	if len(diags) == 0 {
		t.Fatal("expected at least one diagnostic about missing template")
	}

	foundWarning := false
	for _, d := range diags {
		if d.Level == LevelWarning && strings.Contains(d.Message, "nonexistent") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected warning about missing template 'nonexistent', got: %v", diags)
	}

	// Check that no errors were reported
	if collector.HasLevel(LevelError) {
		t.Errorf("expected no errors when using fallback template, got: %v", collector.DiagnosticsAtLevel(LevelError))
	}

	// Check that the output file was created (pretty URLs: test.md -> test/index.html)
	outputFile := filepath.Join(outputDir, "test", "index.html")
	outputContent, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	// Verify the fallback template was used
	if !bytes.Contains(outputContent, []byte("FALLBACK:")) {
		t.Errorf("expected fallback template to be used, got: %s", string(outputContent))
	}

	// Verify Meta fields are populated
	if !bytes.Contains(outputContent, []byte("Source:")) {
		t.Error("expected Meta.Source to be in output")
	}
	if !bytes.Contains(outputContent, []byte("Target:")) {
		t.Error("expected Meta.Target to be in output")
	}
	if !bytes.Contains(outputContent, []byte("Template: nonexistent")) {
		t.Error("expected Meta.Template to show the missing template name")
	}
	if !bytes.Contains(outputContent, []byte("Dev: true")) {
		t.Error("expected SiteMeta.Dev to be true")
	}
}

func TestStepContent_NoFallbackTemplate_ProdMode(t *testing.T) {
	// Create a temporary directory for our test
	tmpDir, err := os.MkdirTemp("", "shizuka-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create directory structure
	contentDir := filepath.Join(tmpDir, "content")
	templatesDir := filepath.Join(tmpDir, "templates")
	outputDir := filepath.Join(tmpDir, "dist")

	for _, dir := range []string{contentDir, templatesDir, outputDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create a content file with a missing template
	contentFile := filepath.Join(contentDir, "test.md")
	content := `---
title: "Test Page"
template: "nonexistent"
date: 2024-01-01
---

# Hello World
`
	if err := os.WriteFile(contentFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write content file: %v", err)
	}

	// Create a basic template (but not the one referenced)
	templateFile := filepath.Join(templatesDir, "page.html")
	templateContent := `<!DOCTYPE html><html><body>{{ .Page.Body }}</body></html>`
	if err := os.WriteFile(templateFile, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template file: %v", err)
	}

	// Create config
	config := &Config{
		Site: SiteConfig{
			Title: "Test Site",
		},
		Build: BuildConfig{
			ContentDir:    contentDir,
			StaticDir:     filepath.Join(tmpDir, "static"),
			TemplatesGlob: filepath.Join(templatesDir, "*.html"),
			OutputDir:     outputDir,
		},
	}

	os.MkdirAll(config.Build.StaticDir, 0755)

	// Create diagnostic collector
	collector := NewDiagnosticCollector()

	// Run build WITHOUT fallback template (prod mode)
	steps := []Step{
		StepContent(),
	}

	opts := []Option{
		WithContext(context.Background()),
		WithDiagnosticSink(collector),
		// No fallback template
		// No lenient errors
	}

	err = Build(steps, config, opts...)

	// Build should fail because there's an error-level diagnostic
	if err == nil {
		t.Fatal("expected build to fail without fallback template in prod mode")
	}

	// Check that an error was reported
	if !collector.HasLevel(LevelError) {
		t.Errorf("expected error diagnostic about missing template")
	}
}
