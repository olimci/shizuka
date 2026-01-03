package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/olimci/shizuka/pkg/build"
)

type buildOutputStyle int

const (
	buildOutputPlain buildOutputStyle = iota
	buildOutputRich
)

type logPrinter struct {
	style buildOutputStyle
	out   io.Writer
	mu    sync.Mutex

	levelStyles map[build.DiagnosticLevel]lipgloss.Style
	stepStyle   lipgloss.Style
	sourceStyle lipgloss.Style
}

func newLogPrinter(style buildOutputStyle, out io.Writer) *logPrinter {
	p := &logPrinter{
		style: style,
		out:   out,
	}

	if style != buildOutputRich {
		return p
	}

	colorEnabled := false
	if f, ok := out.(*os.File); ok {
		colorEnabled = isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
	}
	if !colorEnabled {
		return p
	}

	p.levelStyles = map[build.DiagnosticLevel]lipgloss.Style{
		build.LevelDebug:   lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")), // muted
		build.LevelInfo:    lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")), // blue
		build.LevelWarning: lipgloss.NewStyle().Foreground(lipgloss.Color("#f9e2af")), // yellow
		build.LevelError:   lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")), // red
	}
	p.stepStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6adc8"))   // grey
	p.sourceStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#cdd6f4")) // text
	return p
}

func (p *logPrinter) Print(d build.Diagnostic) {
	p.mu.Lock()
	defer p.mu.Unlock()

	line := formatLogPlain(d)
	if p.style == buildOutputRich && p.levelStyles != nil {
		levelStyle, ok := p.levelStyles[d.Level]
		if ok {
			levelToken := levelStyle.Render(d.Level.String())
			line = formatLogRich(d, levelToken, p.stepStyle, p.sourceStyle)
		}
	}

	fmt.Fprintln(p.out, line)
}

func formatLogPlain(d build.Diagnostic) string {
	var b strings.Builder

	b.WriteString(d.Level.String())
	if d.StepID != "" {
		b.WriteString(" [")
		b.WriteString(d.StepID)
		b.WriteString("]")
	}
	b.WriteString(": ")

	if d.Source != "" {
		b.WriteString(d.Source)
		b.WriteString(": ")
	}

	b.WriteString(d.Message)
	if d.Err != nil {
		b.WriteString(": ")
		b.WriteString(d.Err.Error())
	}

	return b.String()
}

func formatLogRich(d build.Diagnostic, levelToken string, stepStyle, sourceStyle lipgloss.Style) string {
	var b strings.Builder

	b.WriteString(levelToken)
	if d.StepID != "" {
		b.WriteString(" ")
		b.WriteString(stepStyle.Render("[" + d.StepID + "]"))
	}
	b.WriteString(": ")

	if d.Source != "" {
		b.WriteString(sourceStyle.Render(d.Source))
		b.WriteString(": ")
	}

	b.WriteString(d.Message)
	if d.Err != nil {
		b.WriteString(": ")
		b.WriteString(d.Err.Error())
	}

	return b.String()
}
