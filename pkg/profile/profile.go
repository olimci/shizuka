package profile

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"sync"
	"time"

	"github.com/olimci/shizuka/pkg/utils/fileutil"
)

func NewState() *State {
	return &State{
		enabled: true,
		started: make(map[*Span]struct{}),
	}
}

type State struct {
	enabled bool

	mu      sync.Mutex
	start   time.Time
	spans   []*Span
	started map[*Span]struct{}
}

type Report struct {
	Start time.Time     `json:"start"`
	End   time.Time     `json:"end"`
	Total time.Duration `json:"total_ns"`

	Spans []*Span `json:"spans"`
}

type Span struct {
	state *State

	Name     string `json:"name"`
	Category string `json:"category"`

	StartAbs time.Time     `json:"start_abs"`
	EndAbs   time.Time     `json:"end_abs"`
	StartRel time.Duration `json:"start_rel_ns"`
	EndRel   time.Duration `json:"end_rel_ns"`

	Args map[string]string `json:"args,omitempty"`

	once sync.Once
}

func (s *State) Begin() {
	if s == nil || !s.enabled {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.start = time.Now()
	s.spans = s.spans[:0]
	clear(s.started)
}

func (s *State) StartSpan(name, category string, args map[string]string) *Span {
	if s == nil || !s.enabled {
		return noopSpan
	}

	if args == nil {
		args = map[string]string{}
	}

	span := &Span{
		state:    s,
		Name:     name,
		Category: category,
		StartAbs: time.Now(),
		Args:     args,
	}

	s.mu.Lock()
	s.spans = append(s.spans, span)
	s.started[span] = struct{}{}
	s.mu.Unlock()

	return span
}

func (s *State) Finalise() *Report {
	if s == nil || !s.enabled {
		return &Report{}
	}

	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	report := &Report{
		Start: s.start,
		End:   now,
		Total: now.Sub(s.start),
		Spans: make([]*Span, 0, len(s.spans)),
	}

	for started := range s.started {
		started.EndAbs = now
		started.EndRel = started.EndAbs.Sub(started.StartAbs)
		delete(s.started, started)
	}

	for _, span := range s.spans {
		final := &Span{
			Name:     span.Name,
			Category: span.Category,
			StartAbs: span.StartAbs,
			EndAbs:   span.EndAbs,
			StartRel: span.StartAbs.Sub(s.start),
			EndRel:   span.EndAbs.Sub(s.start),
			Args:     maps.Clone(span.Args),
		}
		report.Spans = append(report.Spans, final)
	}

	return report
}

func (s *Span) End(args map[string]string) {
	if s == nil {
		return
	}

	s.once.Do(func() {
		now := time.Now()

		state := s.state
		if state == nil || !state.enabled {
			return
		}

		state.mu.Lock()
		defer state.mu.Unlock()

		s.EndAbs = now
		s.EndRel = s.EndAbs.Sub(s.StartAbs)

		maps.Copy(s.Args, args)

		delete(state.started, s)
	})
}

var noopSpan = &Span{}

func (s *Span) Disabled() bool {
	return s == noopSpan
}

func WriteJSON(path string, report *Report) error {
	if path == "" {
		return nil
	}
	if report == nil {
		report = &Report{}
	}

	dir := filepath.Dir(path)
	if err := fileutil.EnsureDir(dir); err != nil {
		return fmt.Errorf("profile output %q: %w", path, err)
	}

	_, err := fileutil.AtomicWriteWithOptions(path, func(w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return fmt.Errorf("encode profile report: %w", err)
		}
		return nil
	}, fileutil.AtomicOptions{})
	return err
}
