package server

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fsnotify/fsnotify"
	"github.com/olimci/shizuka/pkg/config"
)

func NewWatcher(configPath string, debounce time.Duration) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &Watcher{
		watcher:    w,
		debounce:   debounce,
		configPath: configPath,
		Events:     make(chan WatchEvent, 64),
		Errors:     make(chan error, 64),
	}, nil
}

type Watcher struct {
	Events chan WatchEvent
	Errors chan error

	watcher  *fsnotify.Watcher
	debounce time.Duration

	configPath string
	watched    map[string]struct{}
}

type WatchEvent struct {
	Reason string
	Paths  []string
}

func (w *Watcher) Start(ctx context.Context) error {
	w.watched = make(map[string]struct{})
	if err := w.addPath(w.configPath); err != nil {
		return fmt.Errorf("config file %q: %w", w.configPath, err)
	}
	if cfg, err := config.Load(w.configPath); err == nil {
		paths, globs, err := cfg.WatchedPaths()
		if err != nil {
			lazySend(w.Errors, err)
		}
		if err := w.addPaths(paths...); err != nil {
			lazySend(w.Errors, err)
		}
		if err := w.addGlobs(globs...); err != nil {
			lazySend(w.Errors, err)
		}
	}

	go w.loop(ctx)
	return nil
}

func (w *Watcher) Close() error {
	if w.watcher == nil {
		return nil
	}
	return w.watcher.Close()
}

func (w *Watcher) loop(ctx context.Context) {
	var (
		timer   *time.Timer
		timerCh <-chan time.Time
		pending = make(map[string]struct{})
	)
	defer close(w.Events)
	defer close(w.Errors)
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	resetTimer := func() {
		if timer == nil {
			timer = time.NewTimer(w.debounce)
			timerCh = timer.C
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(w.debounce)
	}

	flush := func(reason string) {
		if len(pending) == 0 {
			return
		}
		paths := make([]string, 0, len(pending))
		for p := range pending {
			paths = append(paths, p)
			delete(pending, p)
		}
		lazySend(w.Events, WatchEvent{Reason: reason, Paths: paths})
	}

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			if ev.Op&fsnotify.Chmod == fsnotify.Chmod {
				continue
			}
			if w.isConfigEvent(ev) {
				w.rebuildWatches()
			}
			if ev.Op&fsnotify.Create == fsnotify.Create {
				w.addDirectoryIfNeeded(ev.Name)
			}
			pending[ev.Name] = struct{}{}
			resetTimer()
		case <-timerCh:
			timer = nil
			timerCh = nil
			flush(fmt.Sprintf("file change (%s quiet)", w.debounce))
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			lazySend(w.Errors, err)
		}
	}
}

func (w *Watcher) addPath(root string) error {
	if w.watched == nil {
		w.watched = make(map[string]struct{})
	}
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return w.addWatch(root)
	}
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return w.addWatch(p)
	})
}

func (w *Watcher) addPaths(paths ...string) error {
	for _, p := range paths {
		if err := w.addPath(p); err != nil {
			return fmt.Errorf("watched path %q: %w", p, err)
		}
	}
	return nil
}

func (w *Watcher) addGlob(pattern string) error {
	files, err := doublestar.FilepathGlob(pattern)
	if err != nil {
		return fmt.Errorf("watched glob %q: %w", pattern, err)
	}
	for _, file := range files {
		if err := w.addPath(file); err != nil {
			return fmt.Errorf("watched glob %q matched %q: %w", pattern, file, err)
		}
	}
	return nil
}

func (w *Watcher) addGlobs(patterns ...string) error {
	for _, glob := range patterns {
		if err := w.addGlob(glob); err != nil {
			return err
		}
	}
	return nil
}

func (w *Watcher) addWatch(p string) error {
	if w.watched == nil {
		w.watched = make(map[string]struct{})
	}
	normalized := filepath.Clean(p)
	if _, ok := w.watched[normalized]; ok {
		return nil
	}
	if err := w.watcher.Add(normalized); err != nil {
		return fmt.Errorf("watch path %q: %w", normalized, err)
	}
	w.watched[normalized] = struct{}{}
	return nil
}

func (w *Watcher) removeAllWatches() {
	if w.watched == nil {
		return
	}
	for p := range w.watched {
		if err := w.watcher.Remove(p); err != nil {
			lazySend(w.Errors, fmt.Errorf("watch path %q: %w", p, err))
		}
	}
	clear(w.watched)
}

func (w *Watcher) rebuildWatches() {
	cfg, err := config.Load(w.configPath)
	if err != nil {
		lazySend(w.Errors, err)
		return
	}
	paths, globs, err := cfg.WatchedPaths()
	if err != nil {
		lazySend(w.Errors, err)
		return
	}

	w.removeAllWatches()
	if err := w.addPath(w.configPath); err != nil {
		lazySend(w.Errors, fmt.Errorf("config file %q: %w", w.configPath, err))
	}
	if err := w.addPaths(paths...); err != nil {
		lazySend(w.Errors, err)
	}
	if err := w.addGlobs(globs...); err != nil {
		lazySend(w.Errors, err)
	}
}

func (w *Watcher) isConfigEvent(ev fsnotify.Event) bool {
	if w.configPath == "" {
		return false
	}
	return filepath.Clean(ev.Name) == filepath.Clean(w.configPath)
}

func (w *Watcher) addDirectoryIfNeeded(p string) {
	info, err := os.Stat(p)
	if err != nil || !info.IsDir() {
		return
	}
	if err := w.addPath(p); err != nil {
		lazySend(w.Errors, fmt.Errorf("directory %q: %w", p, err))
	}
}
