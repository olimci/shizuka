package manifest

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/olimci/shizuka/internal/config"
	"github.com/olimci/shizuka/internal/options"
	"github.com/olimci/shizuka/internal/utils/fileutil"
	"github.com/olimci/shizuka/internal/utils/pool"
)

var ErrConflicts = errors.New("conflicts")
var ErrClosed = errors.New("manifest closed")
var ErrStarted = errors.New("manifest already started")

// New creates a manifest for one build.
func New() *Manifest {
	return &Manifest{}
}

// Manifest writes generated artefacts to a build output directory.
type Manifest struct {
	mu sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
	pool   *pool.Pool

	out     string
	outRoot *os.Root
	options *options.Options
	report  func(Claim, error)

	closed   bool
	finished bool
	started  bool

	claims  map[string][]Claim
	outputs map[string]struct{}
}

// Start opens the output tree and starts accepting artefacts.
func (m *Manifest) Start(ctx context.Context, cfg *config.Config, opts *options.Options, report func(Claim, error), out string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts == nil {
		opts = options.DefaultOptions()
	}

	switch {
	case out != "":
	case opts.OutputPath != "":
		out = opts.OutputPath
	case cfg == nil:
		return errors.New("manifest output path requires config or explicit output path")
	default:
		out = filepath.Join(cfg.Root, filepath.FromSlash(cfg.Paths.Output))
	}
	if err := validateOutputPath(cfg, opts, out); err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return fmt.Errorf("directory %q: %w", out, err)
	}
	if info, err := os.Stat(out); err != nil {
		return fmt.Errorf("directory %q: %w", out, err)
	} else if !info.IsDir() {
		return fmt.Errorf("path %q is not a directory", out)
	}
	outRoot, err := os.OpenRoot(out)
	if err != nil {
		return err
	}
	if !opts.Force {
		empty, err := rootEmpty(outRoot)
		if err != nil {
			_ = outRoot.Close()
			return fmt.Errorf("output %q: %w", out, err)
		}
		if !empty {
			_ = outRoot.Close()
			return fmt.Errorf("output directory %q is not empty; pass --force to overwrite it", out)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		_ = outRoot.Close()
		return ErrStarted
	}

	runCtx, cancel := context.WithCancel(ctx)

	m.ctx = runCtx
	m.cancel = cancel
	m.pool = pool.New(runCtx, opts.MaxWorkers)
	m.out = out
	m.outRoot = outRoot
	m.options = opts
	m.report = report
	m.claims = make(map[string][]Claim)
	m.outputs = make(map[string]struct{})
	m.started = true
	return nil
}

// Emit submits an artefact for manifest processing.
func (m *Manifest) Emit(artefact Artefact) error {
	m.mu.Lock()
	if !m.started || m.closed {
		m.mu.Unlock()
		return ErrClosed
	}
	if err := m.ctx.Err(); err != nil {
		m.mu.Unlock()
		return err
	}
	pool := m.pool
	m.mu.Unlock()

	return pool.Go(func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		accepted, err := m.accept(artefact)
		if err != nil {
			return err
		}
		return m.write(accepted)
	})
}

// Finish closes the manifest, waits for accepted artefacts to drain, and
// reconciles the output tree.
func (m *Manifest) Finish(success bool) error {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return nil
	}
	if m.finished {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	m.finished = true
	pool := m.pool
	cancel := m.cancel
	outRoot := m.outRoot
	dev := m.options != nil && m.options.Dev
	m.mu.Unlock()

	runErr := pool.Wait()

	var err error
	switch {
	case !success && !dev:
		err = m.cleanup(nil)
	case !success:
		err = nil
	case runErr != nil && !dev:
		if cleanupErr := m.cleanup(nil); cleanupErr != nil {
			err = errors.Join(runErr, cleanupErr)
		} else {
			err = runErr
		}
	case runErr != nil:
		err = runErr
	default:
		err = m.cleanup(m.outputSnapshot())
	}

	cancel()
	_ = outRoot.Close()
	return err
}

func (m *Manifest) accept(artefact Artefact) (Artefact, error) {
	target, err := normalizeTarget(artefact.Claim.Target)
	if err != nil {
		return Artefact{}, m.recordError(artefact.Claim, err)
	}
	artefact.Claim.Target = target

	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return Artefact{}, ErrClosed
	}
	if err := m.ctx.Err(); err != nil {
		m.mu.Unlock()
		return Artefact{}, err
	}

	m.claims[target] = append(m.claims[target], artefact.Claim)
	if len(m.claims[target]) > 1 {
		err := conflictError(target, m.claims[target])
		m.mu.Unlock()
		return Artefact{}, m.recordError(NewInternalClaim("manifest", target), err)
	}
	m.outputs[target] = struct{}{}
	m.mu.Unlock()

	return artefact, nil
}

func (m *Manifest) write(artefact Artefact) error {
	target := artefact.Claim.Target
	dir := path.Dir(target)
	if dir == "." {
		dir = ""
	}
	if dir != "" {
		if err := m.outRoot.MkdirAll(dir, 0o755); err != nil {
			return m.recordError(artefact.Claim, err)
		}
	}
	if err := ensureRootDir(m.outRoot, dir); err != nil {
		return m.recordError(artefact.Claim, err)
	}

	exists, err := rootFileExists(m.outRoot, target)
	if err != nil {
		return m.recordError(artefact.Claim, err)
	}

	_, err = fileutil.AtomicWrite(m.outRoot, target, artefact.Builder, fileutil.AtomicOptions{
		Sync:            m.options.SyncWrites,
		CompareExisting: exists,
	})
	if err == nil {
		return nil
	}

	return m.recordError(artefact.Claim, err)
}

func (m *Manifest) outputSnapshot() map[string]struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make(map[string]struct{}, len(m.outputs))
	for target := range m.outputs {
		out[target] = struct{}{}
	}
	return out
}

func (m *Manifest) cleanup(wantFiles map[string]struct{}) error {
	if wantFiles == nil {
		wantFiles = map[string]struct{}{}
	}

	var gotFiles []string
	var gotDirs []string
	err := fs.WalkDir(m.outRoot.FS(), ".", func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			gotDirs = append(gotDirs, filePath)
		} else {
			gotFiles = append(gotFiles, filePath)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("output %q: %w", m.out, err)
	}

	wantDirs := manifestDirs(wantFiles)
	for _, rel := range gotFiles {
		if _, ok := wantFiles[rel]; ok {
			continue
		}
		if err := m.outRoot.Remove(rel); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("output %q: %w", filepath.Clean(filepath.Join(m.out, rel)), err)
		}
	}

	for _, rel := range gotDirs {
		if _, ok := wantDirs[rel]; ok {
			continue
		}
		if err := m.outRoot.RemoveAll(rel); err != nil {
			return fmt.Errorf("output %q: %w", filepath.Clean(filepath.Join(m.out, rel)), err)
		}
	}
	return nil
}

func (m *Manifest) recordError(claim Claim, err error) error {
	if err == nil {
		return nil
	}
	if m.report != nil {
		m.report(claim, err)
	}
	return err
}
