package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/olimci/shizuka/pkg/utils/fileutils"
	"golang.org/x/sync/errgroup"
)

var ErrConflicts = errors.New("conflicts")

func New() *Manifest {
	return &Manifest{
		artefacts: make([]Artefact, 0),
		registry:  make(map[string]any),
	}
}

type Manifest struct {
	// artefacts is a list of artefacts that will be built to files on Build()
	artefacts   []Artefact
	artefactsMu sync.Mutex

	// registry is a store to be used by build steps to share data
	registry   map[string]any
	registryMu sync.RWMutex
}

func (m *Manifest) Set(k string, v any) {
	m.registryMu.Lock()
	defer m.registryMu.Unlock()

	m.registry[k] = v
}

func (m *Manifest) Get(k string) (any, bool) {
	m.registryMu.RLock()
	defer m.registryMu.RUnlock()

	v, ok := m.registry[k]
	return v, ok
}

func (m *Manifest) Emit(a Artefact) {
	m.artefactsMu.Lock()
	defer m.artefactsMu.Unlock()

	m.artefacts = append(m.artefacts, a)
}

func (m *Manifest) Build(opts ...Option) error {
	o := defaultOptions().apply(opts...)

	m.artefactsMu.Lock()
	defer m.artefactsMu.Unlock()

	artefacts, conflicts := makeArtefacts(m.artefacts)
	if !o.ignoreConflicts && len(conflicts) > 0 {
		return fmt.Errorf("%w: %v", ErrConflicts, conflicts)
	}

	info, err := os.Stat(o.BuildDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(o.BuildDir, 0755); err != nil {
				return fmt.Errorf("failed to create build dir %q: %w", o.BuildDir, err)
			}
		} else {
			return fmt.Errorf("failed to stat build dir %q: %w", o.BuildDir, err)
		}
	} else if !info.IsDir() {
		return fmt.Errorf("build dir %q is not a directory", o.BuildDir)
	}

	gotFiles, gotDirs, err := fileutils.Walk(o.BuildDir)
	if err != nil {
		return fmt.Errorf("walk dist: %w", err)
	}

	cleaned := make(map[string]ArtefactBuilder, len(artefacts))
	for dest, a := range artefacts {
		rel := filepath.Clean(dest)
		if filepath.IsAbs(rel) || isRel(rel) {
			return fmt.Errorf("unsafe artefact path %q escapes dist", dest)
		}
		cleaned[rel] = a
	}
	artefacts = cleaned

	wantDirs := manifestDirs(artefacts)

	for _, rel := range gotFiles.Values() {
		if _, wants := artefacts[rel]; !wants {
			if err := os.Remove(filepath.Join(o.BuildDir, rel)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("failed to remove %s: %w", filepath.Join(o.BuildDir, rel), err)
			}
		}
	}

	for _, rel := range gotDirs.Values() {
		if !wantDirs.Has(rel) {
			if err := os.RemoveAll(filepath.Join(o.BuildDir, rel)); err != nil {
				return fmt.Errorf("failed to remove %s: %w", filepath.Join(o.BuildDir, rel), err)
			}
		}
	}

	for _, rel := range wantDirs.Values() {
		full := filepath.Join(o.BuildDir, rel)
		if !gotDirs.Has(rel) {
			if err := os.MkdirAll(full, 0o755); err != nil {
				return fmt.Errorf("failed to create %s: %w", full, err)
			}
		}
	}

	g, ctx := errgroup.WithContext(o.Context)
	if o.maxWorkers > 0 {
		g.SetLimit(o.maxWorkers)
	}

	for target, artefact := range artefacts {
		exists := gotFiles.Has(target)
		full := filepath.Join(o.BuildDir, target)

		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if exists {
				if err := fileutils.AtomicEdit(full, artefact); err != nil {
					return fmt.Errorf("failed to edit %s: %w", full, err)
				}
			} else {
				if err := fileutils.AtomicWrite(full, artefact); err != nil {
					return fmt.Errorf("failed to write %s: %w", full, err)
				}
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("failed to build: %w", err)
	}

	return nil
}
