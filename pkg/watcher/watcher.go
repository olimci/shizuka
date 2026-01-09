package watcher

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
	"github.com/olimci/shizuka/pkg/utils/set"
)

func New(configPath string, debounce time.Duration) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &Watcher{
		watcher:    w,
		debounce:   debounce,
		configPath: configPath,
		Events:     make(chan Event, 64),
		Errors:     make(chan error, 64),
	}, nil
}

type Watcher struct {
	Events chan Event
	Errors chan error

	watcher  *fsnotify.Watcher
	debounce time.Duration

	configPath string
	watched    *set.Set[string]
}

func (w *Watcher) Start(ctx context.Context) error {
	w.watched = set.New[string]()
	w.addPath(w.configPath)
	if cfg, err := config.Load(w.configPath); err == nil {
		paths, globs := cfg.WatchedPaths()
		if err := w.addPaths(paths...); err != nil {
			lazySend(w.Errors, fmt.Errorf("failed to add paths: %w", err))
		}
		if err := w.addGlobs(globs...); err != nil {
			lazySend(w.Errors, fmt.Errorf("failed to add globs: %w", err))
		}
	}

	go w.loop(ctx)

	return nil
}

func (w *Watcher) loop(ctx context.Context) {
	var (
		timer     *time.Timer
		timerCh   <-chan time.Time
		pending   = set.New[string]()
		lastEvent time.Time
	)

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
		if pending.Len() == 0 {
			return
		}
		paths := make([]string, 0, pending.Len())
		for _, p := range pending.Values() {
			paths = append(paths, p)
		}
		pending.Clear()

		lazySend(w.Events, Event{Reason: reason, Paths: paths})
	}

	for {
		select {
		case <-ctx.Done():
			return

		case ev := <-w.watcher.Events:
			if ev.Op&fsnotify.Chmod == fsnotify.Chmod {
				continue
			}
			if w.isConfigEvent(ev) {
				w.rebuild()
			}
			if ev.Op&fsnotify.Create == fsnotify.Create {
				w.addDirectoryIfNeeded(ev.Name)
			}
			lastEvent = time.Now()
			pending.Add(ev.Name)
			resetTimer()

		case <-timerCh:
			timer = nil
			timerCh = nil
			reason := "file change"
			if !lastEvent.IsZero() {
				reason = fmt.Sprintf("file change (%s quiet)", w.debounce)
			}
			flush(reason)

		case err := <-w.watcher.Errors:
			lazySend(w.Errors, fmt.Errorf("watch error: %w", err))
		}
	}
}

func (w *Watcher) Close() error {
	if w.watcher == nil {
		return nil
	}
	return w.watcher.Close()
}

func (w *Watcher) addPath(root string) error {
	if w.watched == nil {
		w.watched = set.New[string]()
	}
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return w.addWatch(root)
	}

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		return w.addWatch(path)
	})
}

func (w *Watcher) addPaths(paths ...string) error {
	for _, path := range paths {
		if err := w.addPath(path); err != nil {
			return err
		}
	}
	return nil
}

func (w *Watcher) addGlob(pattern string) error {
	files, err := doublestar.Glob(os.DirFS("."), pattern)
	if err != nil {
		return err
	}

	for _, file := range files {
		if err := w.addPath(file); err != nil {
			return err
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

func (w *Watcher) addWatch(path string) error {
	if w.watched == nil {
		w.watched = set.New[string]()
	}
	normalized := filepath.Clean(path)
	if w.watched.Has(normalized) {
		return nil
	}
	if err := w.watcher.Add(normalized); err != nil {
		return err
	}
	w.watched.Add(normalized)
	return nil
}

func (w *Watcher) removeAllWatches() {
	if w.watched == nil {
		return
	}
	for _, path := range w.watched.Values() {
		if err := w.watcher.Remove(path); err != nil {
			lazySend(w.Errors, fmt.Errorf("failed to remove watch: %w", err))
		}
	}
	w.watched.Clear()
}

func (w *Watcher) rebuild() {
	cfg, err := config.Load(w.configPath)
	if err != nil {
		lazySend(w.Errors, fmt.Errorf("failed to reload config: %w", err))
		return
	}
	paths, globs := cfg.WatchedPaths()

	w.removeAllWatches()
	if err := w.addPath(w.configPath); err != nil {
		lazySend(w.Errors, fmt.Errorf("failed to watch config: %w", err))
	}
	if err := w.addPaths(paths...); err != nil {
		lazySend(w.Errors, fmt.Errorf("failed to add paths: %w", err))
	}
	if err := w.addGlobs(globs...); err != nil {
		lazySend(w.Errors, fmt.Errorf("failed to add globs: %w", err))
	}
}

func (w *Watcher) isConfigEvent(ev fsnotify.Event) bool {
	if w.configPath == "" {
		return false
	}
	return filepath.Clean(ev.Name) == filepath.Clean(w.configPath)
}

func (w *Watcher) addDirectoryIfNeeded(path string) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return
	}
	if err := w.addPath(path); err != nil {
		lazySend(w.Errors, fmt.Errorf("failed to watch new directory: %w", err))
	}
}
