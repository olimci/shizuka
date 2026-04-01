package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/olimci/coffee"
	"github.com/olimci/shizuka/pkg/build"
)

func formatDiagnostic(err *build.BuildError) string {
	if err == nil {
		return ""
	}

	if target := err.Target(); target != "" {
		if source := err.Source(); source != "" && target != source {
			return fmt.Sprintf("%s [%s]", err.Description(), target)
		}
		return err.Description()
	}

	if owner := err.Owner(); owner != "" {
		return fmt.Sprintf("%s: %s", owner, err.Description())
	}

	return err.Description()
}

func formatBuildError(err error) []string {
	if err == nil {
		return nil
	}

	var failure *build.Failure
	if errors.As(err, &failure) && failure.HasErrors() {
		grouped := make(map[string][]*build.BuildError)
		order := make([]string, 0)

		for _, buildErr := range failure.Errors {
			key := buildErr.Source()
			if key == "" {
				key = buildErr.Location()
			}
			if key == "" {
				key = "build"
			}
			if _, ok := grouped[key]; !ok {
				order = append(order, key)
			}
			grouped[key] = append(grouped[key], buildErr)
		}

		lines := make([]string, 0, len(failure.Errors)+len(order))
		for _, key := range order {
			lines = append(lines, coffee.InverseErrorStyle.Render(" "+key+" "))
			for _, buildErr := range grouped[key] {
				for _, line := range strings.Split(formatDiagnostic(buildErr), "\n") {
					lines = append(lines, "  "+coffee.ErrorStyle.Render(line))
				}
			}
		}
		return lines
	}

	return []string{err.Error()}
}
