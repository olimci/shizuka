package cmd

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestCompletionSuggestsCommandsAndFlags(t *testing.T) {
	out := runCLI(t, []string{"shizuka", "--generate-shell-completion"})
	assertContains(t, out, "build:Build site")
	assertContains(t, out, "dev:Start development server")

	out = runCLI(t, []string{"shizuka", "--fo", "--generate-shell-completion"})
	assertContains(t, out, "--format:Output format: auto, plain, pretty, or json")
}

func TestCompletionPrintsShellScripts(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "pwsh"} {
		t.Run(shell, func(t *testing.T) {
			out := runCLI(t, []string{"shizuka", "completion", shell})
			assertContains(t, out, "completion")
		})
	}
}

func runCLI(t *testing.T, args []string) string {
	t.Helper()

	var out bytes.Buffer
	app := newRootCommand()
	app.Writer = &out

	oldArgs := os.Args
	os.Args = args
	defer func() {
		os.Args = oldArgs
	}()

	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("Run(%q) error = %v", strings.Join(args, " "), err)
	}
	return out.String()
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("output = %q, want substring %q", got, want)
	}
}
