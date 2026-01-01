package build

import (
	"fmt"
	"slices"
	"strings"
	"sync"
)

type DiagnosticLevel int

const (
	LevelDebug DiagnosticLevel = iota
	LevelInfo
	LevelWarning
	LevelError
)

func (l DiagnosticLevel) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarning:
		return "warning"
	case LevelError:
		return "error"
	default:
		return "unknown"
	}
}

func ParseLevel(s string) (DiagnosticLevel, error) {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug, nil
	case "info":
		return LevelInfo, nil
	case "warning", "warn":
		return LevelWarning, nil
	case "error", "err":
		return LevelError, nil
	default:
		return LevelDebug, fmt.Errorf("unknown level: %s", s)
	}
}

type Diagnostic struct {
	Level   DiagnosticLevel // Diagnostic level
	StepID  string          // Which step reported this
	Source  string          // File path or other context
	Message string          // Diagnostic message
	Err     error           // Original error, if any
}

func (d Diagnostic) Error() string {
	if d.Source != "" {
		if d.Err != nil {
			return fmt.Sprintf("[%s] %s: %s: %v", d.Level, d.Source, d.Message, d.Err)
		}
		return fmt.Sprintf("[%s] %s: %s", d.Level, d.Source, d.Message)
	}
	if d.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", d.Level, d.Message, d.Err)
	}
	return fmt.Sprintf("[%s] %s", d.Level, d.Message)
}

type DiagnosticSink interface {
	// Report a diagnostic
	Report(d Diagnostic)
	// Diagnostics returns all diagnostics reported to the sink.
	Diagnostics() []Diagnostic
	// DiagnosticsAtLevel returns all diagnostics reported to the sink at the given level.
	DiagnosticsAtLevel(level DiagnosticLevel) []Diagnostic
	// HasLevel returns true if the sink has any diagnostics at the given level.
	HasLevel(level DiagnosticLevel) bool
	// MaxLevel returns the maximum level of diagnostics reported to the sink.
	MaxLevel() DiagnosticLevel
	// Clear clears all diagnostics reported to the sink.
	Clear()
}

// DiagnosticCollector is a DiagnosticSink that collects diagnostics in memory.
type DiagnosticCollector struct {
	mu          sync.RWMutex
	diagnostics []Diagnostic
	minLevel    DiagnosticLevel

	OnReport func(Diagnostic)
}

type CollectorOption func(*DiagnosticCollector)

func WithMinLevel(level DiagnosticLevel) CollectorOption {
	return func(c *DiagnosticCollector) {
		c.minLevel = level
	}
}

func WithOnReport(fn func(Diagnostic)) CollectorOption {
	return func(c *DiagnosticCollector) {
		c.OnReport = fn
	}
}

func NewDiagnosticCollector(opts ...CollectorOption) *DiagnosticCollector {
	c := &DiagnosticCollector{
		minLevel: LevelDebug, // Collect everything by default
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *DiagnosticCollector) Report(d Diagnostic) {
	if d.Level < c.minLevel {
		return
	}

	c.mu.Lock()
	c.diagnostics = append(c.diagnostics, d)
	callback := c.OnReport
	c.mu.Unlock()

	if callback != nil {
		callback(d)
	}
}

func (c *DiagnosticCollector) Diagnostics() []Diagnostic {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return slices.Clone(c.diagnostics)
}

func (c *DiagnosticCollector) DiagnosticsAtLevel(level DiagnosticLevel) []Diagnostic {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var result []Diagnostic
	for _, d := range c.diagnostics {
		if d.Level >= level {
			result = append(result, d)
		}
	}
	return result
}

func (c *DiagnosticCollector) HasLevel(level DiagnosticLevel) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, d := range c.diagnostics {
		if d.Level >= level {
			return true
		}
	}
	return false
}

func (c *DiagnosticCollector) MaxLevel() DiagnosticLevel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.diagnostics) == 0 {
		return DiagnosticLevel(-1)
	}
	max := LevelDebug
	for _, d := range c.diagnostics {
		if d.Level > max {
			max = d.Level
		}
	}
	return max
}

func (c *DiagnosticCollector) Clear() {
	c.mu.Lock()
	c.diagnostics = nil
	c.mu.Unlock()
}

func (c *DiagnosticCollector) CountByLevel() map[DiagnosticLevel]int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	counts := make(map[DiagnosticLevel]int)
	for _, d := range c.diagnostics {
		counts[d.Level]++
	}
	return counts
}

func (c *DiagnosticCollector) Summary() string {
	counts := c.CountByLevel()
	if len(counts) == 0 {
		return "no diagnostics"
	}

	var parts []string
	for _, level := range []DiagnosticLevel{LevelError, LevelWarning, LevelInfo, LevelDebug} {
		if count := counts[level]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d %s(s)", count, level))
		}
	}
	return strings.Join(parts, ", ")
}

// noopSink is a DiagnosticSink that does nothing.
type noopSink struct{}

func (noopSink) Report(Diagnostic)                               {}
func (noopSink) Diagnostics() []Diagnostic                       { return nil }
func (noopSink) DiagnosticsAtLevel(DiagnosticLevel) []Diagnostic { return nil }
func (noopSink) HasLevel(DiagnosticLevel) bool                   { return false }
func (noopSink) MaxLevel() DiagnosticLevel                       { return DiagnosticLevel(-1) }
func (noopSink) Clear()                                          {}

// NoopSink returns a DiagnosticSink that does nothing.
func NoopSink() DiagnosticSink {
	return noopSink{}
}
