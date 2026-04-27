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
	"time"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/options"
	"github.com/olimci/shizuka/pkg/profile"
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

	profileState *profile.State

	ctx    context.Context
	cancel context.CancelFunc

	writeCh chan queuedArtefact
	wg      sync.WaitGroup

	mu       sync.Mutex
	firstErr error
	closed   bool
	queueLen int

	claims      map[string]*claimState
	conflicts   map[string][]Claim
	createdDirs map[string]struct{}
}

type claimState struct {
	artefact Artefact
	discard  bool
}

type queuedArtefact struct {
	target   string
	state    *claimState
	enqueued time.Time
}

func (m *Manifest) Begin(ctx context.Context, config *config.Config, options *options.Options, report func(Claim, error), out string, profileState *profile.State) error {
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

	rt := &runtimeState{
		out:          out,
		options:      options,
		report:       report,
		profileState: profileState,
		ctx:          runCtx,
		cancel:       cancel,
		writeCh:      make(chan queuedArtefact, max(4, workers*2)),
		claims:       make(map[string]*claimState),
		conflicts:    make(map[string][]Claim),
		createdDirs:  make(map[string]struct{}),
	}
	m.runtime = rt

	for i := 0; i < workers; i++ {
		rt.wg.Add(1)
		go func(id int) {
			defer rt.wg.Done()
			rt.writerLoop(id)
		}(i + 1)
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

	end := rt.startSpan("register claim", "manifest-coordinator", map[string]any{
		"owner":  a.Claim.Owner,
		"target": a.Claim.Target,
	})
	defer end(nil)

	target, err := normalizeTarget(a.Claim.Target)
	if err != nil {
		rt.recordError(a.Claim, err)
		end(map[string]any{"result": "invalid"})
		return
	}
	a.Claim.Target = target

	if conflictErr := rt.registerClaim(a.Claim); conflictErr != nil {
		rt.recordError(NewInternalClaim("manifest", target), conflictErr)
		end(map[string]any{"result": "conflict"})
		return
	}

	state := &claimState{artefact: a}
	rt.mu.Lock()
	rt.claims[target] = state
	rt.queueLen++
	rt.mu.Unlock()

	select {
	case <-rt.ctx.Done():
		rt.recordRuntimeError(rt.ctx.Err())
	case rt.writeCh <- queuedArtefact{
		target:   target,
		state:    state,
		enqueued: time.Now(),
	}:
		end(map[string]any{"result": "queued"})
	}
}

func (m *Manifest) Complete(success bool) error {
	m.artefactsMu.Lock()
	rt := m.runtime
	m.artefactsMu.Unlock()

	if rt == nil {
		return nil
	}

	rt.closeQueue()
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

func (rt *runtimeState) writerLoop(workerID int) {
	track := fmt.Sprintf("manifest-writer-%d", workerID)
	for {
		select {
		case <-rt.ctx.Done():
			return
		case item, ok := <-rt.writeCh:
			if !ok {
				return
			}

			rt.mu.Lock()
			rt.queueLen--
			rt.mu.Unlock()

			waitEnd := rt.startSpan("queue wait", track, map[string]any{
				"target": item.target,
			})
			waitEnd(map[string]any{
				"queued_for_us": time.Since(item.enqueued).Microseconds(),
			})

			rt.writeOne(workerID, item, track)
		}
	}
}

func (rt *runtimeState) writeOne(workerID int, item queuedArtefact, track string) {
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

	end := rt.startSpan("write artefact", track, map[string]any{
		"target": item.target,
		"owner":  item.state.artefact.Claim.Owner,
		"worker": workerID,
	})

	var (
		changed bool
		err     error
	)
	if exists {
		changed, err = fileutil.AtomicEditWithOptions(full, item.state.artefact.Builder, fileutil.AtomicOptions{
			Sync: rt.options.SyncWrites,
		})
	} else {
		changed, err = fileutil.AtomicWriteWithOptions(full, item.state.artefact.Builder, fileutil.AtomicOptions{
			Sync: rt.options.SyncWrites,
		})
	}
	if err != nil {
		var discardErr *errutil.DiscardError
		if errors.As(err, &discardErr) {
			item.state.discard = true
			end(map[string]any{"result": "discard"})
			if removeErr := os.Remove(full); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
				rt.recordError(item.state.artefact.Claim, removeErr)
			}
			return
		}
		end(map[string]any{"result": "error"})
		rt.recordError(item.state.artefact.Claim, err)
		return
	}

	result := "skipped"
	if changed {
		result = "written"
	}
	end(map[string]any{"result": result})
}

func (rt *runtimeState) cleanup() error {
	if rt.options.SkipOutputCleanup {
		return nil
	}

	endWalk := rt.startSpan("walk destination", "manifest-coordinator", nil)
	gotFiles, gotDirs, err := fileutil.Walk(rt.out)
	endWalk(nil)
	if err != nil {
		return fmt.Errorf("output %q: %w", displayPath(rt.out, "."), err)
	}

	wantFiles := rt.activeTargets()
	wantDirs := manifestDirs(wantFiles)

	for _, rel := range gotFiles.Values() {
		if _, ok := wantFiles[rel]; ok {
			continue
		}
		end := rt.startSpan("cleanup stale file", "manifest-coordinator", map[string]any{
			"path": rel,
			"kind": "file",
		})
		err := os.Remove(filepath.Join(rt.out, rel))
		end(nil)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("output %q: %w", displayPath(rt.out, rel), err)
		}
	}

	for _, rel := range gotDirs.Values() {
		if wantDirs.Has(rel) {
			continue
		}
		end := rt.startSpan("cleanup stale dir", "manifest-coordinator", map[string]any{
			"path": rel,
			"kind": "dir",
		})
		err := os.RemoveAll(filepath.Join(rt.out, rel))
		end(nil)
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

	end := rt.startSpan("mkdir", "manifest-coordinator", map[string]any{"path": full})
	err := os.MkdirAll(full, 0o755)
	end(nil)
	if err != nil {
		return err
	}

	rt.mu.Lock()
	rt.createdDirs[full] = struct{}{}
	rt.mu.Unlock()
	return nil
}

func (rt *runtimeState) registerClaim(claim Claim) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

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
		rt.cancel()
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
	rt.cancel()
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
	close(rt.writeCh)
	rt.closed = true
}

func (rt *runtimeState) startSpan(name, category string, args map[string]any) func(map[string]any) {
	if rt == nil || rt.profileState == nil {
		return func(map[string]any) {}
	}

	span := rt.profileState.StartSpan(name, category, stringifyArgs(args))
	return func(extra map[string]any) {
		span.End(stringifyArgs(extra))
	}
}

func stringifyArgs(args map[string]any) map[string]string {
	if len(args) == 0 {
		return nil
	}

	out := make(map[string]string, len(args))
	for k, v := range args {
		out[k] = fmt.Sprint(v)
	}
	return out
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
