package manifest

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/utils/fileutil"
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
func (m *Manifest) Build(config *config.Config, options *config.Options, report func(Claim, error), out string) error {
	m.artefactsMu.Lock()
	defer m.artefactsMu.Unlock()

	artefacts, conflicts := makeArtefacts(m.artefacts)
	for conflict, files := range conflicts {
		owners := make([]string, 0, len(files))
		for _, file := range files {
			owner := strings.TrimSpace(file.Owner)
			if owner == "" {
				owner = strings.TrimSpace(file.Source)
			}
			if owner == "" {
				owner = strings.TrimSpace(file.Target)
			}
			if owner == "" {
				owner = "<unknown>"
			}
			owners = append(owners, owner)
		}

		if report != nil {
			report(
				NewInternalClaim("manifest", conflict),
				fmt.Errorf("%w for %q: claimed by %s", ErrConflicts, conflict, strings.Join(owners, ", ")),
			)
		}
	}
	if !options.Dev && len(conflicts) > 0 {
		return fmt.Errorf("%w: %d conflicting output path(s)", ErrConflicts, len(conflicts))
	}

	// Ensure we have a valid destination
	if strings.TrimSpace(out) == "" {
		out = config.Resolved.Build.Output
		if options.OutputPath != "" {
			out = options.OutputPath
		}
	}

	if err := ensureOutputRoot(out); err != nil {
		return err
	}

	gotFiles, gotDirs, err := walkDestination(out)
	if err != nil {
		return fmt.Errorf("output %q: %w", displayPath(out, "."), err)
	}

	cleaned := make(map[string]Artefact, len(artefacts))
	for dest, a := range artefacts {
		rel := path.Clean(filepath.ToSlash(dest))
		if path.IsAbs(rel) || isRel(rel) {
			return fmt.Errorf("output path %q escapes the build output root", dest)
		}
		a.Claim.Target = rel
		cleaned[rel] = a
	}
	artefacts = cleaned

	wantDirs := manifestDirs(artefacts)

	for _, rel := range gotFiles.Values() {
		if _, wants := artefacts[rel]; !wants {
			if err := os.Remove(filepath.Join(out, rel)); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("output %q: %w", displayPath(out, rel), err)
			}
		}
	}

	for _, rel := range gotDirs.Values() {
		if !wantDirs.Has(rel) {
			if err := os.RemoveAll(filepath.Join(out, rel)); err != nil {
				return fmt.Errorf("output %q: %w", displayPath(out, rel), err)
			}
		}
	}

	for _, rel := range wantDirs.Values() {
		if !gotDirs.Has(rel) {
			if err := os.MkdirAll(filepath.Join(out, rel), 0o755); err != nil {
				return fmt.Errorf("output %q: %w", displayPath(out, rel), err)
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

			full := filepath.Join(out, target)
			var err error
			if exists {
				err = fileutil.AtomicEdit(full, artefact.Builder)
			} else {
				err = fileutil.AtomicWrite(full, artefact.Builder)
			}
			if err != nil {
				if report != nil {
					report(artefact.Claim, err)
				}
				return err
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}
