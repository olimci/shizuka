package internal

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/olimci/shizuka/pkg/build"
)

type DevServer struct {
	builder *Builder
	server  *Server
	watcher *FileWatcher
	ui      *UI
}

type DevServerConfig struct {
	ConfigPath string
	DistDir    string
	Port       int
	Debounce   time.Duration
	NoUI       bool
	WatchPaths []string
}

type DevServerEvent struct {
	Type    string
	Message string
	Data    any
}

func NewDevServer(config DevServerConfig) (*DevServer, error) {
	builder, err := NewBuilderWithDistOverride(config.ConfigPath, config.DistDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create builder: %w", err)
	}

	server := NewServer(ServerConfig{
		DistDir: config.DistDir,
		Port:    config.Port,
	})

	watcher, err := NewFileWatcher(WatcherConfig{
		Paths:    config.WatchPaths,
		Debounce: config.Debounce,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	ui := NewUI(!config.NoUI)

	return &DevServer{
		builder: builder,
		server:  server,
		watcher: watcher,
		ui:      ui,
	}, nil
}

func (ds *DevServer) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	baseURL, err := ds.server.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	watchEvents, watchErrors, err := ds.watcher.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start file watcher: %w", err)
	}

	buildRequests := make(chan BuildRequest, 10)
	buildResults := make(chan BuildResult, 10)
	uiEvents := make(chan tea.Msg, 10)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		ds.buildWorker(ctx, buildRequests, buildResults, uiEvents)
	}()

	select {
	case buildRequests <- BuildRequest{Reason: "initial", Paths: nil}:
	default:
		ds.ui.LogEvent("build skipped: request queue full")
	}

	if ds.ui.IsInteractive() {
		return ds.runWithUI(ctx, baseURL, buildRequests, watchEvents, watchErrors, buildResults, uiEvents, &wg)
	} else {
		return ds.runWithoutUI(ctx, baseURL, buildRequests, watchEvents, watchErrors, buildResults, uiEvents, &wg)
	}
}

func (ds *DevServer) runWithUI(ctx context.Context, baseURL string, buildRequests chan<- BuildRequest, watchEvents <-chan WatchEvent, watchErrors <-chan error, buildResults <-chan BuildResult, uiEvents chan tea.Msg, wg *sync.WaitGroup) error {
	model := ds.ui.NewModel(baseURL, buildRequests)
	program := tea.NewProgram(model)

	done := make(chan struct{})
	var runErr error

	go func() {
		defer close(done)
		_, runErr = program.Run()
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-watchEvents:
				select {
				case buildRequests <- BuildRequest{Reason: event.Reason, Paths: event.Paths}:
				default:
					ds.ui.LogEvent("rebuild skipped: request queue full")
				}
			case err := <-watchErrors:
				ds.ui.LogEvent(fmt.Sprintf("watch error: %v", err))
			case result := <-buildResults:
				msg := ds.ui.BuildResultToMsg(result)
				select {
				case uiEvents <- msg:
				default:
				}
			}
		}
	}()

	select {
	case <-done:
		wg.Wait()
		return runErr
	case <-ctx.Done():
		program.Quit()
		<-done
		wg.Wait()
		return ctx.Err()
	}
}

func (ds *DevServer) runWithoutUI(ctx context.Context, baseURL string, buildRequests chan<- BuildRequest, watchEvents <-chan WatchEvent, watchErrors <-chan error, buildResults <-chan BuildResult, uiEvents chan tea.Msg, wg *sync.WaitGroup) error {
	log.Printf("shizuka dev server started")
	log.Printf("baseURL: %s", baseURL)
	log.Printf("watching: %s", strings.Join(ds.watcher.paths, ", "))

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()

		case event := <-watchEvents:
			if event.Reason == "watcher started" {
				log.Printf("watching: %s", strings.Join(event.Paths, ", "))
				continue
			}
			select {
			case buildRequests <- BuildRequest{Reason: event.Reason, Paths: event.Paths}:
			default:
				log.Print("rebuild skipped: request queue full")
			}

		case err := <-watchErrors:
			log.Printf("watch error: %v", err)

		case result := <-buildResults:
			ds.logBuildResult(result)

		case msg := <-uiEvents:
			ds.ui.PrintMsg(msg)
		}
	}
}

func (ds *DevServer) buildWorker(ctx context.Context, requests <-chan BuildRequest, results chan<- BuildResult, events chan tea.Msg) {
	buildCount := 0

	for {
		select {
		case <-ctx.Done():
			return
		case req := <-requests:
			buildCount++

			startMsg := BuildStartedMsg{
				Reason: req.Reason,
				Number: buildCount,
			}
			select {
			case events <- startMsg:
			default:
			}

			var buildResult BuildResult
			if req.Reason == "initial" {
				buildResult = ds.builder.Build(ctx)
			} else {
				buildResult = ds.builder.BuildDev(ctx)
			}

			enhancedResult := BuildResult{
				Duration:    buildResult.Duration,
				Error:       buildResult.Error,
				Reason:      req.Reason,
				Paths:       req.Paths,
				Number:      buildCount,
				Diagnostics: buildResult.Diagnostics,
			}

			select {
			case results <- enhancedResult:
			default:
			}
		}
	}
}

func (ds *DevServer) logBuildResult(result BuildResult) {
	for _, d := range result.Diagnostics {
		prefix := levelPrefixLog(d.Level)
		if d.Source != "" {
			log.Printf("%s %s: %s", prefix, d.Source, d.Message)
		} else {
			log.Printf("%s %s", prefix, d.Message)
		}
	}

	if result.Error != nil {
		log.Printf("ERR  build #%d failed in %s (%s): %v", result.Number, result.Duration.Truncate(time.Millisecond), result.Reason, result.Error)
		if len(result.Paths) > 0 {
			log.Printf("     changes: %s", strings.Join(result.Paths, ", "))
		}
		return
	}

	summary := ds.summarizeDiagnostics(result.Diagnostics)
	if summary != "" {
		log.Printf("OK   build #%d in %s (%s) [%s]", result.Number, result.Duration.Truncate(time.Millisecond), result.Reason, summary)
	} else {
		log.Printf("OK   build #%d in %s (%s)", result.Number, result.Duration.Truncate(time.Millisecond), result.Reason)
	}
	if len(result.Paths) > 0 {
		log.Printf("     changes: %s", strings.Join(result.Paths, ", "))
	}
}

func levelPrefixLog(level build.DiagnosticLevel) string {
	switch level {
	case build.LevelDebug:
		return "DBG "
	case build.LevelInfo:
		return "INFO"
	case build.LevelWarning:
		return "WARN"
	case build.LevelError:
		return "ERR "
	default:
		return "    "
	}
}

func (ds *DevServer) summarizeDiagnostics(diagnostics []build.Diagnostic) string {
	counts := make(map[build.DiagnosticLevel]int)
	for _, d := range diagnostics {
		counts[d.Level]++
	}

	if len(counts) == 0 {
		return ""
	}

	var parts []string
	for _, level := range []build.DiagnosticLevel{build.LevelError, build.LevelWarning, build.LevelInfo, build.LevelDebug} {
		if count := counts[level]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", count, level))
		}
	}
	return strings.Join(parts, ", ")
}

func (ds *DevServer) Close() error {
	var errs []error

	if err := ds.watcher.Close(); err != nil {
		errs = append(errs, fmt.Errorf("watcher close: %w", err))
	}

	if err := ds.server.Shutdown(); err != nil {
		errs = append(errs, fmt.Errorf("server shutdown: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

type BuildRequest struct {
	Reason string
	Paths  []string
	Number int
}

type BuildStartedMsg struct {
	Reason string
	Number int
}
