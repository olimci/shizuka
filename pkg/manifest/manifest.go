package manifest

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/options"
	"github.com/olimci/shizuka/pkg/utils/errutil"
	"github.com/olimci/shizuka/pkg/utils/fileutil"
)

var ErrConflicts = errors.New("conflicts")

// New creates a new manifest
func New() *Manifest {
	return &Manifest{
		artefacts: make([]Artefact, 0),
	}
}

// Manifest represents a manifest of build artefacts.
type Manifest struct {
	artefacts   []Artefact
	artefactsMu sync.Mutex

	runtime *runtimeState
}

type runtimeState struct {
	out     string
	options *options.Options
	report  func(Claim, error)

	ctx    context.Context
	cancel context.CancelFunc

	wg sync.WaitGroup

	mu      sync.Mutex
	cond    *sync.Cond
	pending []queuedArtefact
	next    int

	firstErr       error
	closed         bool
	startedWorkers int

	claims      map[string]*claimState
	conflicts   map[string][]Claim
	createdDirs map[string]struct{}
}

type claimState struct {
	artefact Artefact
	discard  bool
}

type queuedArtefact struct {
	target string
	state  *claimState
}

func (m *Manifest) Begin(ctx context.Context, config *config.Config, options *options.Options, report func(Claim, error), out string) error {
	m.artefactsMu.Lock()
	defer m.artefactsMu.Unlock()

	if m.runtime != nil {
		return nil
	}

	if strings.TrimSpace(out) == "" {
		resolvedOut, err := config.OutputPath()
		if err != nil {
			return err
		}
		out = resolvedOut
		if options.OutputPath != "" {
			out = options.OutputPath
		}
	}

	if err := fileutil.EnsureDir(out); err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	workers := options.MaxWorkers
	if workers <= 0 {
		workers = 1
	}
	artefactWorkers := options.ArtefactWorkers
	if artefactWorkers < 0 {
		artefactWorkers = defaultArtefactWorkers(workers)
	}

	rt := &runtimeState{
		out:         out,
		options:     options,
		report:      report,
		ctx:         runCtx,
		cancel:      cancel,
		claims:      make(map[string]*claimState),
		conflicts:   make(map[string][]Claim),
		createdDirs: make(map[string]struct{}),
	}
	rt.cond = sync.NewCond(&rt.mu)
	m.runtime = rt

	if artefactWorkers > 0 {
		rt.startWorkers(artefactWorkers)
	}

	for _, artefact := range m.artefacts {
		m.emitLocked(artefact)
	}
	m.artefacts = nil

	return nil
}

// Emit adds an artefact to the manifest
func (m *Manifest) Emit(a Artefact) {
	m.artefactsMu.Lock()
	defer m.artefactsMu.Unlock()

	if m.runtime == nil {
		m.artefacts = append(m.artefacts, a)
		return
	}

	m.emitLocked(a)
}

func (m *Manifest) emitLocked(a Artefact) {
	rt := m.runtime
	if rt == nil {
		m.artefacts = append(m.artefacts, a)
		return
	}

	target, err := normalizeTarget(a.Claim.Target)
	if err != nil {
		rt.recordError(a.Claim, err)
		return
	}
	a.Claim.Target = target

	state := &claimState{artefact: a}
	rt.mu.Lock()
	if conflictErr := rt.registerClaimLocked(a.Claim); conflictErr != nil {
		rt.mu.Unlock()
		rt.recordError(NewInternalClaim("manifest", target), conflictErr)
		return
	}
	if err := rt.ctx.Err(); err != nil {
		rt.mu.Unlock()
		rt.recordRuntimeError(err)
		return
	}
	rt.claims[target] = state
	rt.pending = append(rt.pending, queuedArtefact{
		target: target,
		state:  state,
	})
	rt.cond.Signal()
	rt.mu.Unlock()
}

func (m *Manifest) Complete(success bool) error {
	m.artefactsMu.Lock()
	rt := m.runtime
	m.artefactsMu.Unlock()

	if rt == nil {
		return nil
	}

	drainWorkers := rt.options.MaxWorkers
	if drainWorkers <= 0 {
		drainWorkers = 1
	}
	rt.closeQueue()
	rt.startWorkers(drainWorkers)
	rt.wg.Wait()

	if err := rt.firstRuntimeError(); err != nil {
		return err
	}
	if !success {
		return nil
	}

	if err := rt.cleanup(); err != nil {
		return err
	}

	return nil
}

func defaultArtefactWorkers(stepWorkers int) int {
	if stepWorkers <= 4 {
		return 1
	}
	return min(2, max(1, stepWorkers/4))
}

func (rt *runtimeState) startWorkers(workers int) {
	if workers <= 0 {
		return
	}
	rt.mu.Lock()
	rt.startedWorkers += workers
	rt.mu.Unlock()

	for i := 0; i < workers; i++ {
		rt.wg.Add(1)
		go func() {
			defer rt.wg.Done()
			rt.writerLoop()
		}()
	}
}

func (rt *runtimeState) writerLoop() {
	for {
		item, ok := rt.nextArtefact()
		if !ok {
			return
		}

		rt.writeOne(item)
	}
}

func (rt *runtimeState) nextArtefact() (queuedArtefact, bool) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	for rt.next >= len(rt.pending) && !rt.closed && rt.ctx.Err() == nil {
		rt.cond.Wait()
	}
	if rt.ctx.Err() != nil {
		return queuedArtefact{}, false
	}
	if rt.next >= len(rt.pending) {
		return queuedArtefact{}, false
	}

	item := rt.pending[rt.next]
	rt.pending[rt.next] = queuedArtefact{}
	rt.next++
	if rt.next > 1024 && rt.next*2 >= len(rt.pending) {
		copy(rt.pending, rt.pending[rt.next:])
		rt.pending = rt.pending[:len(rt.pending)-rt.next]
		rt.next = 0
	}
	return item, true
}

func (rt *runtimeState) writeOne(item queuedArtefact) {
	parent := filepath.Dir(filepath.Join(rt.out, item.target))
	if err := rt.ensureDir(parent); err != nil {
		rt.recordError(item.state.artefact.Claim, err)
		return
	}

	full := filepath.Join(rt.out, item.target)
	_, statErr := os.Stat(full)
	exists := statErr == nil
	if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		rt.recordError(item.state.artefact.Claim, statErr)
		return
	}

	var err error
	if exists {
		_, err = fileutil.AtomicEditWithOptions(full, item.state.artefact.Builder, fileutil.AtomicOptions{
			Sync: rt.options.SyncWrites,
		})
	} else {
		_, err = fileutil.AtomicWriteWithOptions(full, item.state.artefact.Builder, fileutil.AtomicOptions{
			Sync: rt.options.SyncWrites,
		})
	}
	if err != nil {
		var discardErr *errutil.DiscardError
		if errors.As(err, &discardErr) {
			item.state.discard = true
			if removeErr := os.Remove(full); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
				rt.recordError(item.state.artefact.Claim, removeErr)
			}
			return
		}
		rt.recordError(item.state.artefact.Claim, err)
		return
	}
}

func (rt *runtimeState) cleanup() error {
	if rt.options.SkipOutputCleanup {
		return nil
	}

	gotFiles, gotDirs, err := fileutil.Walk(rt.out)
	if err != nil {
		return fmt.Errorf("output %q: %w", displayPath(rt.out, "."), err)
	}

	wantFiles := rt.activeTargets()
	wantDirs := manifestDirs(wantFiles)

	for _, rel := range gotFiles.Values() {
		if _, ok := wantFiles[rel]; ok {
			continue
		}
		err := os.Remove(filepath.Join(rt.out, rel))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("output %q: %w", displayPath(rt.out, rel), err)
		}
	}

	for _, rel := range gotDirs.Values() {
		if wantDirs.Has(rel) {
			continue
		}
		err := os.RemoveAll(filepath.Join(rt.out, rel))
		if err != nil {
			return fmt.Errorf("output %q: %w", displayPath(rt.out, rel), err)
		}
	}

	return nil
}

func (rt *runtimeState) activeTargets() map[string]Artefact {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	out := make(map[string]Artefact, len(rt.claims))
	for target, state := range rt.claims {
		if state == nil || state.discard {
			continue
		}
		out[target] = state.artefact
	}
	return out
}

func (rt *runtimeState) ensureDir(full string) error {
	full = filepath.Clean(full)
	if full == "." || full == rt.out {
		return nil
	}

	rt.mu.Lock()
	if _, ok := rt.createdDirs[full]; ok {
		rt.mu.Unlock()
		return nil
	}
	rt.mu.Unlock()

	if info, err := os.Stat(full); err == nil && info.IsDir() {
		rt.mu.Lock()
		rt.createdDirs[full] = struct{}{}
		rt.mu.Unlock()
		return nil
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	err := os.MkdirAll(full, 0o755)
	if err != nil {
		return err
	}

	rt.mu.Lock()
	rt.createdDirs[full] = struct{}{}
	rt.mu.Unlock()
	return nil
}

func (rt *runtimeState) registerClaimLocked(claim Claim) error {
	target := claim.Target
	if _, exists := rt.claims[target]; !exists {
		return nil
	}

	rt.conflicts[target] = append(rt.conflicts[target], claim)

	owners := make([]string, 0, 2+len(rt.conflicts[target]))
	if existing := rt.claims[target]; existing != nil {
		owners = append(owners, displayOwner(existing.artefact.Claim))
	}
	for _, other := range rt.conflicts[target] {
		owners = append(owners, displayOwner(other))
	}
	err := fmt.Errorf("%w for %q: claimed by %s", ErrConflicts, target, strings.Join(owners, ", "))
	if !rt.options.Dev {
		if rt.firstErr == nil {
			rt.firstErr = err
		}
	}
	return err
}

func (rt *runtimeState) recordError(claim Claim, err error) {
	if err == nil {
		return
	}
	if rt.report != nil {
		rt.report(claim, err)
	}
	rt.recordRuntimeError(err)
}

func (rt *runtimeState) recordRuntimeError(err error) {
	if err == nil {
		return
	}
	rt.setFirstErr(err)
	rt.cancelRuntime()
}

func (rt *runtimeState) cancelRuntime() {
	rt.cancel()
	rt.mu.Lock()
	rt.cond.Broadcast()
	rt.mu.Unlock()
}

func (rt *runtimeState) setFirstErr(err error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.firstErr == nil {
		rt.firstErr = err
	}
}

func (rt *runtimeState) firstRuntimeError() error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.firstErr == nil || errors.Is(rt.firstErr, context.Canceled) {
		return nil
	}
	return rt.firstErr
}

func (rt *runtimeState) closeQueue() {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.closed {
		return
	}
	rt.closed = true
	rt.cond.Broadcast()
}

func normalizeTarget(dest string) (string, error) {
	rel := path.Clean(filepath.ToSlash(dest))
	if path.IsAbs(rel) || isRel(rel) {
		return "", fmt.Errorf("output path %q escapes the build output root", dest)
	}
	return rel, nil
}

func displayOwner(claim Claim) string {
	owner := strings.TrimSpace(claim.Owner)
	if owner == "" {
		owner = strings.TrimSpace(claim.Source)
	}
	if owner == "" {
		owner = strings.TrimSpace(claim.Target)
	}
	if owner == "" {
		owner = "<unknown>"
	}
	return owner
}
