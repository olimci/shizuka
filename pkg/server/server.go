package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/options"
	"github.com/olimci/shizuka/pkg/registry"
)

const reloadPath = "/_shizuka/reload"

type BuildFunc func(...options.Option) error

type Options struct {
	Addr          string
	Watch         bool
	WatchDebounce time.Duration
	Reload        bool
	Logger        *slog.Logger

	BuildOptions []options.Option
	Build        BuildFunc
}

type RebuildRequest struct {
	Reason       string
	ChangedPaths []string
	ResetCache   bool
}

type EventKind string

const (
	EventStarting       EventKind = "starting"
	EventListening      EventKind = "listening"
	EventBuildStarted   EventKind = "build_started"
	EventBuildSucceeded EventKind = "build_succeeded"
	EventBuildFailed    EventKind = "build_failed"
	EventWatchError     EventKind = "watch_error"
	EventServerError    EventKind = "server_error"
	EventClosed         EventKind = "closed"
)

type Event struct {
	Kind     EventKind
	Reason   string
	Addr     string
	URL      string
	Duration time.Duration
	Err      error
}

type Server struct {
	opts Options

	dist       string
	cleanupDir string
	siteURL    string

	cache  *registry.Registry
	hub    *ReloadHub
	static *StaticHandler
	logger *slog.Logger

	httpServer *http.Server
	listener   net.Listener
	watcher    *Watcher

	buildMu sync.Mutex
	closeMu sync.Mutex
	closed  bool

	events chan Event
}

func New(opts Options) (*Server, error) {
	if opts.Addr == "" {
		return nil, errors.New("server address is required")
	}
	if opts.Build == nil {
		return nil, errors.New("build function is required")
	}

	dist, err := os.MkdirTemp("", "shizuka-*")
	if err != nil {
		return nil, err
	}

	return &Server{
		opts:       opts,
		dist:       dist,
		cleanupDir: dist,
		cache:      registry.New(),
		hub:        NewReloadHub(),
		logger:     serverLogger(opts.Logger),
		events:     make(chan Event, 64),
	}, nil
}

func (s *Server) Events() <-chan Event {
	return s.events
}

func (s *Server) URL() string {
	return s.siteURL
}

func (s *Server) OutputPath() string {
	return s.dist
}

func (s *Server) Start(ctx context.Context) error {
	buildOpts := options.DefaultOptions().Apply(s.opts.BuildOptions...)
	cfg, err := config.Load(buildOpts.ConfigPath)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", s.opts.Addr)
	if err != nil {
		return fmt.Errorf("dev server %q: %w", s.opts.Addr, err)
	}
	s.listener = listener
	s.siteURL = s.resolveSiteURL(listener.Addr())

	s.emit(Event{Kind: EventStarting, Addr: listener.Addr().String(), URL: s.siteURL})

	s.static = NewStaticHandler(s.dist, StaticOptions{
		HeadersFile:   headersFile(cfg),
		RedirectsFile: redirectsFile(cfg),
	})

	var root http.Handler = s.static
	mux := http.NewServeMux()
	if s.opts.Reload {
		mux.Handle(reloadPath, s.hub)
		root = ReloadMiddleware(root)
	}
	mux.Handle("/", root)

	s.httpServer = &http.Server{
		Addr:    s.opts.Addr,
		Handler: mux,
	}

	go s.serve(listener)
	s.emit(Event{Kind: EventListening, Addr: listener.Addr().String(), URL: s.siteURL})

	if s.opts.Watch {
		watcher, err := NewWatcher(buildOpts.ConfigPath, s.opts.WatchDebounce)
		if err != nil {
			_ = s.Close()
			return err
		}
		s.watcher = watcher
		if err := watcher.Start(ctx); err != nil {
			_ = s.Close()
			return err
		}
		go s.watch(ctx, watcher)
	}

	go func() {
		<-ctx.Done()
		_ = s.Close()
	}()

	return s.Rebuild(ctx, RebuildRequest{Reason: "initial"})
}

func (s *Server) Rebuild(ctx context.Context, req RebuildRequest) error {
	s.buildMu.Lock()
	defer s.buildMu.Unlock()

	req.ChangedPaths = options.CleanChangedPaths(req.ChangedPaths)
	if req.Reason == "" {
		req.Reason = "manual"
	}
	s.logger.Debug("rebuild requested", "reason", req.Reason, "changed_paths", len(req.ChangedPaths), "reset_cache", req.ResetCache)
	if req.ResetCache {
		s.cache = registry.New()
	}
	s.refreshControlFiles(req.ChangedPaths)

	s.emit(Event{Kind: EventBuildStarted, Reason: req.Reason, URL: s.siteURL})

	start := time.Now()
	err := s.opts.Build(s.buildOptions(ctx, req.ChangedPaths)...)
	elapsed := time.Since(start).Truncate(time.Millisecond)
	if err != nil {
		s.emit(Event{Kind: EventBuildFailed, Reason: req.Reason, URL: s.siteURL, Duration: elapsed, Err: err})
		return nil
	}

	if s.opts.Reload {
		s.hub.Broadcast("reload")
	}
	s.emit(Event{Kind: EventBuildSucceeded, Reason: req.Reason, URL: s.siteURL, Duration: elapsed})
	return nil
}

func (s *Server) refreshControlFiles(changedPaths []string) {
	buildOpts := options.DefaultOptions().Apply(s.opts.BuildOptions...)
	if s.static == nil || !shouldRefreshConfig(buildOpts.ConfigPath, changedPaths) {
		return
	}
	cfg, err := config.Load(buildOpts.ConfigPath)
	if err != nil {
		return
	}
	s.static.SetControlFiles(headersFile(cfg), redirectsFile(cfg))
}

func (s *Server) ResetCache() {
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	s.cache = registry.New()
}

func (s *Server) Close() error {
	s.closeMu.Lock()
	if s.closed {
		s.closeMu.Unlock()
		return nil
	}
	s.closed = true
	s.closeMu.Unlock()

	var errs []error
	if s.watcher != nil {
		errs = append(errs, s.watcher.Close())
	}
	if s.httpServer != nil {
		errs = append(errs, s.httpServer.Close())
	}
	if s.cleanupDir != "" {
		errs = append(errs, os.RemoveAll(s.cleanupDir))
	}

	s.emit(Event{Kind: EventClosed})
	return errors.Join(errs...)
}

func (s *Server) serve(listener net.Listener) {
	err := s.httpServer.Serve(listener)
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return
	}
	s.emit(Event{Kind: EventServerError, Err: err})
}

func (s *Server) watch(ctx context.Context, watcher *Watcher) {
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			s.emit(Event{Kind: EventWatchError, Err: err})
		case ev, ok := <-watcher.Events:
			if !ok {
				return
			}
			if err := s.Rebuild(ctx, RebuildRequest{Reason: ev.Reason, ChangedPaths: ev.Paths}); err != nil {
				s.emit(Event{Kind: EventServerError, Err: err})
			}
		}
	}
}

func (s *Server) buildOptions(ctx context.Context, changedPaths []string) []options.Option {
	internal := options.Filter(
		options.WithContext(ctx),
		options.WithInternalOutputPath(s.dist),
		options.WithInternalSiteURL(s.siteURL),
		options.WithInternalCache(s.cache),
		options.WithInternalChanges(changedPaths),
	)
	all := make([]options.Option, 0, len(s.opts.BuildOptions)+len(internal))
	all = append(all, s.opts.BuildOptions...)
	all = append(all, internal...)
	return all
}

func (s *Server) resolveSiteURL(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "http://localhost/"
	}
	if host == "" || host == "::" || host == "0.0.0.0" || host == "[::]" {
		host = "localhost"
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	if port == "" || port == "0" {
		if tcpAddr, ok := addr.(*net.TCPAddr); ok {
			port = strconv.Itoa(tcpAddr.Port)
		}
	}
	return fmt.Sprintf("http://%s:%s/", host, port)
}

func (s *Server) emit(ev Event) {
	select {
	case s.events <- ev:
	default:
	}
}
