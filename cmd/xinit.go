package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/olimci/shizuka/pkg/scaffold"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

func runXInit(ctx context.Context, cmd *cli.Command) error {
	source, target, err := parseXInitArgs(cmd)
	if err != nil {
		return err
	}

	selected := cmd.String("template")
	force := cmd.Bool("force")
	listOnly := cmd.Bool("list")
	listVars := cmd.Bool("list-vars")
	varPairs := cmd.StringSlice("var")
	varsFile := strings.TrimSpace(cmd.String("vars-file"))

	if listOnly && listVars {
		return fmt.Errorf("--list and --list-vars cannot be used together")
	}

	tmpl, coll, close, err := loadTemplate(ctx, source)
	if err != nil {
		return err
	}
	defer close()

	if listOnly {
		return printTemplateList(tmpl, coll)
	}

	chosen, err := resolveTemplateFromSource(tmpl, coll, selected)
	if err != nil {
		return err
	}

	if listVars {
		return printTemplateVars(chosen)
	}

	vars, err := loadXInitVars(varsFile, varPairs)
	if err != nil {
		return err
	}

	applyTemplateVarDefaults(chosen, vars)

	buildResult, err := chosen.Build(ctx, target,
		scaffold.WithForce(force),
		scaffold.WithVariables(vars),
	)
	if err != nil {
		return err
	}

	fmt.Print(renderBuildResult(buildResult, target))

	return nil
}

func parseXInitArgs(cmd *cli.Command) (string, string, error) {
	sourceFlag := strings.TrimSpace(cmd.String("source"))
	target := "."
	source := ""

	switch cmd.NArg() {
	case 0:
	case 1:
		arg := strings.TrimSpace(cmd.Args().Get(0))
		if sourceFlag != "" {
			target = arg
		} else {
			source = arg
		}
	case 2:
		if sourceFlag != "" {
			return "", "", fmt.Errorf("too many arguments when --source is set")
		}
		source = strings.TrimSpace(cmd.Args().Get(0))
		target = strings.TrimSpace(cmd.Args().Get(1))
	default:
		return "", "", fmt.Errorf("too many arguments!")
	}

	if source == "" {
		source = sourceFlag
	} else if sourceFlag != "" {
		return "", "", fmt.Errorf("source provided both as arg and --source")
	}

	if strings.TrimSpace(target) == "" {
		target = "."
	}

	return source, target, nil
}

func loadXInitVars(varsFile string, pairs []string) (map[string]any, error) {
	vars := make(map[string]any)

	if varsFile != "" {
		fileVars, err := readVarsFile(varsFile)
		if err != nil {
			return nil, err
		}
		maps.Copy(vars, fileVars)
	}

	for _, pair := range pairs {
		key, val, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --var %q (expected key=value)", pair)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("invalid --var %q (empty key)", pair)
		}
		vars[key] = strings.TrimSpace(val)
	}

	return vars, nil
}

func readVarsFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading vars file: %w", err)
	}

	var decoded map[string]any
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".toml":
		if _, err := toml.Decode(string(data), &decoded); err != nil {
			return nil, fmt.Errorf("parsing vars file: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &decoded); err != nil {
			return nil, fmt.Errorf("parsing vars file: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &decoded); err != nil {
			return nil, fmt.Errorf("parsing vars file: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported vars file extension %q", ext)
	}

	if decoded == nil {
		return map[string]any{}, nil
	}

	if nested, ok := decoded["variables"]; ok {
		if asMap, ok := nested.(map[string]any); ok {
			return asMap, nil
		}
	}

	return decoded, nil
}
