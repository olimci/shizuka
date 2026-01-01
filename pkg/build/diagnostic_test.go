package build

import (
	"sync"
	"testing"
)

func TestDiagnosticLevelString(t *testing.T) {
	tests := []struct {
		level    DiagnosticLevel
		expected string
	}{
		{LevelDebug, "debug"},
		{LevelInfo, "info"},
		{LevelWarning, "warning"},
		{LevelError, "error"},
		{DiagnosticLevel(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("DiagnosticLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected DiagnosticLevel
		wantErr  bool
	}{
		{"debug", LevelDebug, false},
		{"DEBUG", LevelDebug, false},
		{"info", LevelInfo, false},
		{"INFO", LevelInfo, false},
		{"warning", LevelWarning, false},
		{"warn", LevelWarning, false},
		{"WARN", LevelWarning, false},
		{"error", LevelError, false},
		{"err", LevelError, false},
		{"ERR", LevelError, false},
		{"invalid", LevelDebug, true},
		{"", LevelDebug, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseLevel(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLevel(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDiagnosticError(t *testing.T) {
	tests := []struct {
		name     string
		diag     Diagnostic
		expected string
	}{
		{
			name: "with source and error",
			diag: Diagnostic{
				Level:   LevelError,
				Source:  "test.go",
				Message: "something failed",
				Err:     ErrNoTemplate,
			},
			expected: "[error] test.go: something failed: no template found",
		},
		{
			name: "with source no error",
			diag: Diagnostic{
				Level:   LevelWarning,
				Source:  "test.go",
				Message: "something warned",
			},
			expected: "[warning] test.go: something warned",
		},
		{
			name: "no source with error",
			diag: Diagnostic{
				Level:   LevelInfo,
				Message: "info message",
				Err:     ErrTemplateNotFound,
			},
			expected: "[info] info message: template not found",
		},
		{
			name: "no source no error",
			diag: Diagnostic{
				Level:   LevelDebug,
				Message: "debug message",
			},
			expected: "[debug] debug message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.diag.Error(); got != tt.expected {
				t.Errorf("Diagnostic.Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDiagnosticCollector_Report(t *testing.T) {
	c := NewDiagnosticCollector()

	c.Report(Diagnostic{Level: LevelDebug, Message: "debug"})
	c.Report(Diagnostic{Level: LevelInfo, Message: "info"})
	c.Report(Diagnostic{Level: LevelWarning, Message: "warning"})
	c.Report(Diagnostic{Level: LevelError, Message: "error"})

	diags := c.Diagnostics()
	if len(diags) != 4 {
		t.Errorf("expected 4 diagnostics, got %d", len(diags))
	}
}

func TestDiagnosticCollector_MinLevel(t *testing.T) {
	c := NewDiagnosticCollector(WithMinLevel(LevelWarning))

	c.Report(Diagnostic{Level: LevelDebug, Message: "debug"})
	c.Report(Diagnostic{Level: LevelInfo, Message: "info"})
	c.Report(Diagnostic{Level: LevelWarning, Message: "warning"})
	c.Report(Diagnostic{Level: LevelError, Message: "error"})

	diags := c.Diagnostics()
	if len(diags) != 2 {
		t.Errorf("expected 2 diagnostics (warning and error), got %d", len(diags))
	}

	for _, d := range diags {
		if d.Level < LevelWarning {
			t.Errorf("expected only warning and error levels, got %v", d.Level)
		}
	}
}

func TestDiagnosticCollector_OnReport(t *testing.T) {
	var reported []Diagnostic
	c := NewDiagnosticCollector(WithOnReport(func(d Diagnostic) {
		reported = append(reported, d)
	}))

	c.Report(Diagnostic{Level: LevelWarning, Message: "test"})

	if len(reported) != 1 {
		t.Errorf("expected OnReport to be called once, got %d calls", len(reported))
	}
	if reported[0].Message != "test" {
		t.Errorf("expected message 'test', got %q", reported[0].Message)
	}
}

func TestDiagnosticCollector_DiagnosticsAtLevel(t *testing.T) {
	c := NewDiagnosticCollector()

	c.Report(Diagnostic{Level: LevelDebug, Message: "debug"})
	c.Report(Diagnostic{Level: LevelInfo, Message: "info"})
	c.Report(Diagnostic{Level: LevelWarning, Message: "warning"})
	c.Report(Diagnostic{Level: LevelError, Message: "error"})

	tests := []struct {
		level    DiagnosticLevel
		expected int
	}{
		{LevelDebug, 4},
		{LevelInfo, 3},
		{LevelWarning, 2},
		{LevelError, 1},
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			diags := c.DiagnosticsAtLevel(tt.level)
			if len(diags) != tt.expected {
				t.Errorf("DiagnosticsAtLevel(%v) returned %d diagnostics, want %d", tt.level, len(diags), tt.expected)
			}
		})
	}
}

func TestDiagnosticCollector_HasLevel(t *testing.T) {
	c := NewDiagnosticCollector()

	c.Report(Diagnostic{Level: LevelWarning, Message: "warning"})

	if !c.HasLevel(LevelDebug) {
		t.Error("HasLevel(LevelDebug) should return true when warning exists")
	}
	if !c.HasLevel(LevelWarning) {
		t.Error("HasLevel(LevelWarning) should return true")
	}
	if c.HasLevel(LevelError) {
		t.Error("HasLevel(LevelError) should return false")
	}
}

func TestDiagnosticCollector_MaxLevel(t *testing.T) {
	t.Run("empty collector", func(t *testing.T) {
		c := NewDiagnosticCollector()
		if got := c.MaxLevel(); got != DiagnosticLevel(-1) {
			t.Errorf("MaxLevel() on empty collector = %v, want -1", got)
		}
	})

	t.Run("with diagnostics", func(t *testing.T) {
		c := NewDiagnosticCollector()
		c.Report(Diagnostic{Level: LevelDebug, Message: "debug"})
		c.Report(Diagnostic{Level: LevelWarning, Message: "warning"})

		if got := c.MaxLevel(); got != LevelWarning {
			t.Errorf("MaxLevel() = %v, want %v", got, LevelWarning)
		}
	})
}

func TestDiagnosticCollector_Clear(t *testing.T) {
	c := NewDiagnosticCollector()

	c.Report(Diagnostic{Level: LevelError, Message: "error"})
	if len(c.Diagnostics()) != 1 {
		t.Fatal("expected 1 diagnostic before clear")
	}

	c.Clear()

	if len(c.Diagnostics()) != 0 {
		t.Error("expected 0 diagnostics after clear")
	}
	if c.HasLevel(LevelDebug) {
		t.Error("HasLevel should return false after clear")
	}
}

func TestDiagnosticCollector_CountByLevel(t *testing.T) {
	c := NewDiagnosticCollector()

	c.Report(Diagnostic{Level: LevelDebug, Message: "debug1"})
	c.Report(Diagnostic{Level: LevelDebug, Message: "debug2"})
	c.Report(Diagnostic{Level: LevelWarning, Message: "warning"})
	c.Report(Diagnostic{Level: LevelError, Message: "error1"})
	c.Report(Diagnostic{Level: LevelError, Message: "error2"})
	c.Report(Diagnostic{Level: LevelError, Message: "error3"})

	counts := c.CountByLevel()

	if counts[LevelDebug] != 2 {
		t.Errorf("expected 2 debug, got %d", counts[LevelDebug])
	}
	if counts[LevelInfo] != 0 {
		t.Errorf("expected 0 info, got %d", counts[LevelInfo])
	}
	if counts[LevelWarning] != 1 {
		t.Errorf("expected 1 warning, got %d", counts[LevelWarning])
	}
	if counts[LevelError] != 3 {
		t.Errorf("expected 3 errors, got %d", counts[LevelError])
	}
}

func TestDiagnosticCollector_Summary(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		c := NewDiagnosticCollector()
		if got := c.Summary(); got != "no diagnostics" {
			t.Errorf("Summary() = %q, want %q", got, "no diagnostics")
		}
	})

	t.Run("with diagnostics", func(t *testing.T) {
		c := NewDiagnosticCollector()
		c.Report(Diagnostic{Level: LevelWarning, Message: "w1"})
		c.Report(Diagnostic{Level: LevelWarning, Message: "w2"})
		c.Report(Diagnostic{Level: LevelError, Message: "e1"})

		summary := c.Summary()
		// Should contain counts for error and warning
		if summary != "1 error(s), 2 warning(s)" {
			t.Errorf("Summary() = %q, want %q", summary, "1 error(s), 2 warning(s)")
		}
	})
}

func TestDiagnosticCollector_Concurrent(t *testing.T) {
	c := NewDiagnosticCollector()
	var wg sync.WaitGroup

	// Spawn multiple goroutines to report diagnostics concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			level := DiagnosticLevel(n % 4)
			c.Report(Diagnostic{Level: level, Message: "concurrent"})
		}(i)
	}

	wg.Wait()

	diags := c.Diagnostics()
	if len(diags) != 100 {
		t.Errorf("expected 100 diagnostics, got %d", len(diags))
	}
}

func TestNoopSink(t *testing.T) {
	sink := NoopSink()

	// Should not panic
	sink.Report(Diagnostic{Level: LevelError, Message: "test"})

	if diags := sink.Diagnostics(); diags != nil {
		t.Error("NoopSink.Diagnostics() should return nil")
	}
	if sink.HasLevel(LevelDebug) {
		t.Error("NoopSink.HasLevel() should return false")
	}
	if sink.MaxLevel() != DiagnosticLevel(-1) {
		t.Error("NoopSink.MaxLevel() should return -1")
	}

	// Should not panic
	sink.Clear()
}

func TestDiagnosticCollector_DiagnosticsReturnsClone(t *testing.T) {
	c := NewDiagnosticCollector()
	c.Report(Diagnostic{Level: LevelError, Message: "test"})

	diags1 := c.Diagnostics()
	diags2 := c.Diagnostics()

	// Modify the first slice
	diags1[0].Message = "modified"

	// The second slice should be unchanged
	if diags2[0].Message != "test" {
		t.Error("Diagnostics() should return a clone, not the original slice")
	}
}
