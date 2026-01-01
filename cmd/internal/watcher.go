package internal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/fsnotify/fsnotify"
)

type FileWatcher struct {
	watcher  *fsnotify.Watcher
	debounce time.Duration
	paths    []string
}

type WatcherConfig struct {
	Paths    []string
	Debounce time.Duration
}

type WatchEvent struct {
	Reason string
	Paths  []string
}

func NewFileWatcher(config WatcherConfig) (*FileWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &FileWatcher{
		watcher:  w,
		debounce: config.Debounce,
		paths:    config.Paths,
	}, nil
}

func (fw *FileWatcher) Start(ctx context.Context) (<-chan WatchEvent, <-chan error, error) {
	eventCh := make(chan WatchEvent, 10)
	errorCh := make(chan error, 10)

	var watchedPaths []string
	for _, path := range fw.paths {
		path = filepath.Clean(path)
		if err := fw.addRecursive(path); err != nil {
			select {
			case errorCh <- fmt.Errorf("watch warn: %s: %w", path, err):
			default:
			}
			continue
		}
		watchedPaths = append(watchedPaths, path)
	}

	if len(watchedPaths) > 0 {
		select {
		case eventCh <- WatchEvent{Reason: "watcher started", Paths: watchedPaths}:
		default:
		}
	}

	go fw.watchLoop(ctx, eventCh, errorCh)

	return eventCh, errorCh, nil
}

func (fw *FileWatcher) Close() error {
	return fw.watcher.Close()
}

func (fw *FileWatcher) watchLoop(ctx context.Context, eventCh chan<- WatchEvent, errorCh chan<- error) {
	var (
		timer     *time.Timer
		timerC    <-chan time.Time
		pending   = make(map[string]struct{})
		lastEvent time.Time
	)

	resetTimer := func() {
		if timer == nil {
			timer = time.NewTimer(fw.debounce)
			timerC = timer.C
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(fw.debounce)
		timerC = timer.C
	}

	flush := func(reason string) {
		if len(pending) == 0 {
			return
		}
		paths := make([]string, 0, len(pending))
		for p := range pending {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		clear(pending)

		select {
		case eventCh <- WatchEvent{Reason: reason, Paths: paths}:
		default:
		}
	}

	for {
		select {
		case <-ctx.Done():
			return

		case ev := <-fw.watcher.Events:
			if ev.Op&fsnotify.Chmod == fsnotify.Chmod {
				continue
			}
			lastEvent = time.Now()
			pending[ev.Name] = struct{}{}
			resetTimer()

		case <-timerC:
			timerC = nil
			reason := "file change"
			if !lastEvent.IsZero() {
				reason = fmt.Sprintf("file change (%s quiet)", fw.debounce)
			}
			flush(reason)

		case err := <-fw.watcher.Errors:
			select {
			case errorCh <- fmt.Errorf("watch error: %w", err):
			default:
			}
		}
	}
}

func (fw *FileWatcher) addRecursive(root string) error {
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fw.watcher.Add(root)
	}

	return filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "node_modules" || base == ".cache" || base == "dist" {
				return filepath.SkipDir
			}
			return fw.watcher.Add(path)
		}
		return nil
	})
}
