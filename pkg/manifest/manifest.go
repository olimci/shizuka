package manifest

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"sync"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/events"
	"github.com/olimci/shizuka/pkg/iofs"
	"golang.org/x/sync/errgroup"
)

var ErrConflicts = errors.New("conflicts")

// K is a typed key
type K[T any] string

// GetAs retrieves a value from the manifest as the specified type. UB for bad keys/types
func GetAs[T any](m *Manifest, k K[T]) T {
	if v, ok := m.Get(string(k)); ok {
		if vt, ok := v.(T); ok {
			return vt
		}
	}
	return *new(T)
}

func SetAs[T any](m *Manifest, k K[T], v T) {
	m.registryMu.Lock()
	defer m.registryMu.Unlock()

	m.registry[string(k)] = v
}

// New creates a new manifest
func New() *Manifest {
	return &Manifest{
		artefacts: make([]Artefact, 0),
		registry:  make(map[string]any),
	}
}

// Manifest represents a manifest of build artefacts, and a registry of build information
type Manifest struct {
	artefacts   []Artefact
	artefactsMu sync.Mutex

	registry   map[string]any
	registryMu sync.RWMutex
}

// Set sets a value in the registry
func (m *Manifest) Set(k string, v any) {
	m.registryMu.Lock()
	defer m.registryMu.Unlock()

	m.registry[k] = v
}

// Get retrieves a value from the registry
func (m *Manifest) Get(k string) (any, bool) {
	m.registryMu.RLock()
	defer m.registryMu.RUnlock()

	v, ok := m.registry[k]
	return v, ok
}

// Emit adds an artefact to the manifest
func (m *Manifest) Emit(a Artefact) {
	m.artefactsMu.Lock()
	defer m.artefactsMu.Unlock()

	m.artefacts = append(m.artefacts, a)
}

// Build builds the manifest
func (m *Manifest) Build(config *config.Config, options *config.Options, handler events.Handler, out iofs.Writable) error {
	m.artefactsMu.Lock()
	defer m.artefactsMu.Unlock()

	artefacts, conflicts := makeArtefacts(m.artefacts)
	for conflict, files := range conflicts {
		handler.Handle(events.Event{
			Level:   events.Error,
			Message: fmt.Sprintf("file conflict %s: %v", conflict, files),
			Error:   fmt.Errorf("%w: %v", ErrConflicts, conflicts),
		})
	}
	if !options.Dev && len(conflicts) > 0 {
		return fmt.Errorf("%w: %v", ErrConflicts, conflicts)
	}

	// Ensure we have a valid destination
	if out == nil {
		dist := config.Build.Output
		if options.OutputPath != "" {
			dist = options.OutputPath
		}
		out = iofs.FromOS(dist)
	}

	if err := out.EnsureRoot(); err != nil {
		return err
	}

	gotFiles, gotDirs, err := walkDestination(options.Context, out)
	if err != nil {
		return fmt.Errorf("walk dist: %w", err)
	}

	cleaned := make(map[string]ArtefactBuilder, len(artefacts))
	for dest, a := range artefacts {
		rel := path.Clean(filepath.ToSlash(dest))
		if path.IsAbs(rel) || isRel(rel) {
			return fmt.Errorf("unsafe artefact path %q escapes dist", dest)
		}
		cleaned[rel] = a
	}
	artefacts = cleaned

	wantDirs := manifestDirs(artefacts)

	for _, rel := range gotFiles.Values() {
		if _, wants := artefacts[rel]; !wants {
			if err := out.Remove(rel); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("failed to remove %s: %w", displayPath(out, rel), err)
			}
		}
	}

	for _, rel := range gotDirs.Values() {
		if !wantDirs.Has(rel) {
			if err := out.RemoveAll(rel); err != nil {
				return fmt.Errorf("failed to remove %s: %w", displayPath(out, rel), err)
			}
		}
	}

	for _, rel := range wantDirs.Values() {
		if !gotDirs.Has(rel) {
			if err := out.MkdirAll(rel, 0o755); err != nil {
				return fmt.Errorf("failed to create %s: %w", displayPath(out, rel), err)
			}
		}
	}

	g, ctx := errgroup.WithContext(options.Context)
	if options.MaxWorkers > 0 {
		g.SetLimit(options.MaxWorkers)
	}

	for target, artefact := range artefacts {
		exists := gotFiles.Has(target)

		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if err := out.Write(target, artefact, exists); err != nil {
				if exists {
					return fmt.Errorf("failed to edit %s: %w", displayPath(out, target), err)
				}
				return fmt.Errorf("failed to write %s: %w", displayPath(out, target), err)
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("failed to build: %w", err)
	}

	return nil
}
