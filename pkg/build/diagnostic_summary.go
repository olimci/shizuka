package build

import (
	"fmt"
	"strings"
)

func summaryDiagnostics(sink DiagnosticSink) string {
	if sink == nil {
		return "no diagnostics"
	}

	diags := sink.Diagnostics()
	if len(diags) == 0 {
		return "no diagnostics"
	}

	counts := make(map[DiagnosticLevel]int)
	for _, d := range diags {
		counts[d.Level]++
	}

	var parts []string
	for _, level := range []DiagnosticLevel{LevelError, LevelWarning, LevelInfo, LevelDebug} {
		if count := counts[level]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d %s(s)", count, level))
		}
	}
	return strings.Join(parts, ", ")
}
