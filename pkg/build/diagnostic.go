package build

import (
	"fmt"
	"slices"
	"strings"
	"sync"
)

// DiagnosticLevel represents the severity of a diagnostic message.
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

// ParseLevel converts a string to a DiagnosticLevel.
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

// Diagnostic represents a single build issue or log message.
type Diagnostic struct {
	Level   DiagnosticLevel
	StepID  string // Which step reported this
	Source  string // File path or other context
	Message string
	Err     error // Original error, if any
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

// DiagnosticSink collects diagnostics during a build.
type DiagnosticSink interface {
	// Report adds a diagnostic to the sink.
	Report(d Diagnostic)

	// Diagnostics returns all collected diagnostics.
	Diagnostics() []Diagnostic

	// DiagnosticsAtLevel returns diagnostics at or above the given level.
	DiagnosticsAtLevel(level DiagnosticLevel) []Diagnostic

	// HasLevel returns true if any diagnostics at or above level were reported.
	HasLevel(level DiagnosticLevel) bool

	// MaxLevel returns the highest severity level reported, or -1 if empty.
	MaxLevel() DiagnosticLevel

	// Clear removes all diagnostics (useful between rebuilds).
	Clear()
}

// DiagnosticCollector is the default thread-safe implementation of DiagnosticSink.
type DiagnosticCollector struct {
	mu          sync.RWMutex
	diagnostics []Diagnostic
	minLevel    DiagnosticLevel

	// OnReport is an optional callback for real-time streaming.
	OnReport func(Diagnostic)
}

// CollectorOption configures a DiagnosticCollector.
type CollectorOption func(*DiagnosticCollector)

// WithMinLevel sets the minimum level for diagnostics to be collected.
// Diagnostics below this level will be ignored.
func WithMinLevel(level DiagnosticLevel) CollectorOption {
	return func(c *DiagnosticCollector) {
		c.minLevel = level
	}
}

// WithOnReport sets a callback that will be called for each diagnostic reported.
// The callback is called outside the lock, so it's safe to do blocking operations.
func WithOnReport(fn func(Diagnostic)) CollectorOption {
	return func(c *DiagnosticCollector) {
		c.OnReport = fn
	}
}

// NewDiagnosticCollector creates a new DiagnosticCollector with the given options.
func NewDiagnosticCollector(opts ...CollectorOption) *DiagnosticCollector {
	c := &DiagnosticCollector{
		minLevel: LevelDebug, // Collect everything by default
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Report adds a diagnostic to the collector.
// Diagnostics below the minimum level are ignored.
func (c *DiagnosticCollector) Report(d Diagnostic) {
	if d.Level < c.minLevel {
		return
	}

	c.mu.Lock()
	c.diagnostics = append(c.diagnostics, d)
	callback := c.OnReport
	c.mu.Unlock()

	// Call outside lock to avoid deadlocks
	if callback != nil {
		callback(d)
	}
}

// Diagnostics returns a copy of all collected diagnostics.
func (c *DiagnosticCollector) Diagnostics() []Diagnostic {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return slices.Clone(c.diagnostics)
}

// DiagnosticsAtLevel returns diagnostics at or above the given level.
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

// HasLevel returns true if any diagnostics at or above level were reported.
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

// MaxLevel returns the highest severity level reported, or -1 if empty.
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

// Clear removes all diagnostics.
func (c *DiagnosticCollector) Clear() {
	c.mu.Lock()
	c.diagnostics = nil
	c.mu.Unlock()
}

// CountByLevel returns a map of level to count.
func (c *DiagnosticCollector) CountByLevel() map[DiagnosticLevel]int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	counts := make(map[DiagnosticLevel]int)
	for _, d := range c.diagnostics {
		counts[d.Level]++
	}
	return counts
}

// Summary returns a human-readable summary of diagnostics.
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

// noopSink is used when no sink is provided.
type noopSink struct{}

func (noopSink) Report(Diagnostic)                               {}
func (noopSink) Diagnostics() []Diagnostic                       { return nil }
func (noopSink) DiagnosticsAtLevel(DiagnosticLevel) []Diagnostic { return nil }
func (noopSink) HasLevel(DiagnosticLevel) bool                   { return false }
func (noopSink) MaxLevel() DiagnosticLevel                       { return DiagnosticLevel(-1) }
func (noopSink) Clear()                                          {}

// NoopSink returns a DiagnosticSink that discards all diagnostics.
func NoopSink() DiagnosticSink {
	return noopSink{}
}
