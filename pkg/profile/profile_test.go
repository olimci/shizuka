package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStateFinaliseCapturesClosedAndOpenSpans(t *testing.T) {
	state := NewState()
	state.Begin()

	closedArgs := map[string]string{"phase": "parse"}
	closed := state.StartSpan("build", "step", closedArgs)
	closed.End(map[string]string{"status": "ok"})

	open := state.StartSpan("render", "step", nil)

	report := state.Finalise()

	if report.Total < 0 {
		t.Fatalf("report.Total = %v, want >= 0", report.Total)
	}
	if len(report.Spans) != 2 {
		t.Fatalf("len(report.Spans) = %d, want 2", len(report.Spans))
	}

	closedArgs["phase"] = "mutated"
	closed.Args["status"] = "changed"

	first := report.Spans[0]
	if first.Name != "build" || first.Args["phase"] != "parse" || first.Args["status"] != "ok" {
		t.Fatalf("first span = %#v, want captured args", first)
	}
	if first.EndAbs.IsZero() {
		t.Fatal("closed span EndAbs is zero")
	}

	second := report.Spans[1]
	if second.Name != "render" {
		t.Fatalf("second span.Name = %q, want %q", second.Name, "render")
	}
	if second.EndAbs.IsZero() {
		t.Fatal("open span was not finalised")
	}

	_ = open
}

func TestNilStateAndDisabledSpanAreSafe(t *testing.T) {
	var state *State

	span := state.StartSpan("ignored", "ignored", nil)
	if !span.Disabled() {
		t.Fatal("span.Disabled() = false, want true")
	}
	span.End(map[string]string{"ignored": "true"})

	report := state.Finalise()
	if len(report.Spans) != 0 {
		t.Fatalf("len(report.Spans) = %d, want 0", len(report.Spans))
	}
}

func TestBeginResetsPriorRunState(t *testing.T) {
	state := NewState()
	state.Begin()
	span := state.StartSpan("first", "step", nil)
	span.End(nil)

	state.Begin()
	second := state.StartSpan("second", "step", nil)
	second.End(nil)

	report := state.Finalise()
	if len(report.Spans) != 1 {
		t.Fatalf("len(report.Spans) = %d, want 1", len(report.Spans))
	}
	if report.Spans[0].Name != "second" {
		t.Fatalf("report.Spans[0].Name = %q, want %q", report.Spans[0].Name, "second")
	}
}

func TestWriteJSONCreatesOutputFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles", "build.json")

	report := &Report{
		Spans: []*Span{{
			Name:     "build",
			Category: "step",
		}},
	}

	if err := WriteJSON(path, report); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	text := string(data)
	if !strings.Contains(text, "\"spans\"") || !strings.Contains(text, "\"name\": \"build\"") {
		t.Fatalf("profile JSON missing expected fields: %s", text)
	}
}
