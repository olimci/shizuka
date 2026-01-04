package cmd

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/olimci/shizuka/pkg/build"
	"github.com/olimci/shizuka/pkg/utils/fileutils"
	"github.com/urfave/cli/v3"
)

type devLogger func(string)

func runDevHeadless(ctx context.Context, cmd *cli.Command) error {
	configPath, cfg, port, err := loadDevConfig(cmd)
	if err != nil {
		return err
	}

	logger := func(msg string) {
		fmt.Println(msg)
	}

	return runDevServer(ctx, configPath, cfg, port, devServerHooks{
		Log: logger,
	})
}

func loadDevConfig(cmd *cli.Command) (string, *build.Config, int, error) {
	port := cmd.Int("port")
	configPath, cfg, err := loadBuildConfig(cmd)
	if err != nil {
		return "", nil, 0, err
	}

	return configPath, cfg, port, nil
}

type devServerHooks struct {
	Log     devLogger
	Working func(string) error
	Idle    func(string) error
	Message func(string) error
}

func runDevServer(ctx context.Context, configPath string, cfg *build.Config, port int, hooks devServerHooks) error {
	fallbackTmpl, errPageTmpl, buildFailedTmpl, err := loadDevErrTemplates()
	if err != nil {
		return err
	}
	notFoundTmpl, err := load404Template()
	if err != nil {
		return err
	}

	logLine := hooks.Log
	if logLine == nil {
		logLine = func(string) {}
	}

	reloadHub := newReloadHub()
	fileHandler := newDevFileHandler(cfg.Build.OutputDir, reloadHub, notFoundTmpl)
	mux := http.NewServeMux()
	mux.HandleFunc("/_shizuka/reload", reloadHub.ServeHTTP)
	mux.Handle("/", fileHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return err
	}
	serverErrs := make(chan error, 1)
	go func() {
		serverErrs <- server.Serve(listener)
	}()

	url := fmt.Sprintf("http://localhost:%d", port)
	logLine(fmt.Sprintf("Serving on %s", url))
	if hooks.Message != nil {
		_ = hooks.Message(fmt.Sprintf("Serving on %s", url))
	}

	buildOnce := func(reason string) error {
		if hooks.Working != nil {
			_ = hooks.Working(fmt.Sprintf("Building (%s)", reason))
		} else {
			logLine(fmt.Sprintf("Building (%s)", reason))
		}

		collector := build.NewDiagnosticCollector(
			build.WithMinLevel(build.LevelDebug),
			build.WithOnReport(func(d build.Diagnostic) {
				if d.Level < build.LevelInfo {
					return
				}
				line := formatLogPlain(d)
				logLine(line)
				if hooks.Message != nil {
					_ = hooks.Message(line)
				}
			}),
		)

		opts := []build.Option{
			build.WithContext(ctx),
			build.WithConfig(configPath),
			build.WithDiagnosticSink(collector),
			build.WithDev(),
			build.WithErrPages(map[error]*template.Template{
				build.ErrNoTemplate:       fallbackTmpl,
				build.ErrTemplateNotFound: fallbackTmpl,
				build.ErrPageBuild:        errPageTmpl,
			}, errPageTmpl),
			build.WithDevFailurePage(buildFailedTmpl),
		}

		start := time.Now()
		err := build.BuildSteps(defaultBuildSteps(), cfg, opts...)
		elapsed := time.Since(start).Truncate(time.Millisecond)

		if err != nil {
			logLine(fmt.Sprintf("Build failed (%s)", elapsed))
			if collector.HasLevel(build.LevelInfo) {
				logLine(fmt.Sprintf("logs: %s", collector.Summary()))
			}
			if hooks.Idle != nil {
				_ = hooks.Idle("Build failed - watching for changes")
			}
			return err
		}

		logLine(fmt.Sprintf("Build complete (%s)", elapsed))
		if collector.HasLevel(build.LevelInfo) {
			logLine(fmt.Sprintf("logs: %s", collector.Summary()))
		}
		if hooks.Idle != nil {
			_ = hooks.Idle("Watching for changes")
		}
		reloadHub.Broadcast("reload")
		return nil
	}

	if err := buildOnce("initial"); err != nil {
		logLine("Initial build failed; watching for changes.")
	}

	watcher, err := fileutils.NewFileWatcher(fileutils.WatcherConfig{
		Paths:    devWatchPaths(configPath, cfg),
		Debounce: 200 * time.Millisecond,
	})
	if err != nil {
		return err
	}
	defer watcher.Close()

	events, errorsCh, err := watcher.Start(ctx)
	if err != nil {
		return err
	}

	buildPending := false
	building := false
	pendingReason := ""

	triggerBuild := func(reason string) {
		buildPending = true
		if pendingReason == "" {
			pendingReason = reason
		}
	}

	for {
		if buildPending && !building {
			reason := pendingReason
			buildPending = false
			building = true
			pendingReason = ""
			if reason == "" {
				reason = "change"
			}
			if err := buildOnce(reason); err != nil {
				logLine("Waiting for changes to retry.")
			}
			building = false
			continue
		}

		select {
		case <-ctx.Done():
			_ = server.Close()
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}
			return ctx.Err()

		case err := <-serverErrs:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}

		case ev := <-events:
			logLine(fmt.Sprintf("%s: %s", ev.Reason, strings.Join(ev.Paths, ", ")))
			triggerBuild(ev.Reason)

		case err := <-errorsCh:
			if err != nil {
				logLine(err.Error())
			}
		}
	}
}

func devWatchPaths(configPath string, cfg *build.Config) []string {
	paths := []string{
		strings.TrimSpace(configPath),
		strings.TrimSpace(cfg.Build.ContentDir),
		strings.TrimSpace(cfg.Build.StaticDir),
	}

	templates := strings.TrimSpace(cfg.Build.TemplatesGlob)
	if templates != "" {
		paths = append(paths, filepath.Dir(templates))
	}

	seen := make(map[string]struct{}, len(paths))
	unique := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		p = filepath.Clean(p)
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		unique = append(unique, p)
	}

	return unique
}

type reloadHub struct {
	mu      sync.Mutex
	nextID  int
	clients map[int]chan string
}

func newReloadHub() *reloadHub {
	return &reloadHub{
		clients: make(map[int]chan string),
	}
}

func (h *reloadHub) Broadcast(message string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.clients {
		select {
		case ch <- message:
		default:
		}
	}
}

func (h *reloadHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := h.subscribe()
	defer h.unsubscribe(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			_, _ = w.Write([]byte("data: " + msg + "\n\n"))
			flusher.Flush()
		}
	}
}

func (h *reloadHub) subscribe() chan string {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextID++
	ch := make(chan string, 8)
	h.clients[h.nextID] = ch
	return ch
}

func (h *reloadHub) unsubscribe(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, candidate := range h.clients {
		if candidate == ch {
			delete(h.clients, id)
			close(ch)
			return
		}
	}
}

type devFileHandler struct {
	root     string
	reload   *reloadHub
	notFound *template.Template
	files    http.Handler
}

func newDevFileHandler(root string, reload *reloadHub, notFound *template.Template) http.Handler {
	return &devFileHandler{
		root:     root,
		reload:   reload,
		notFound: notFound,
		files:    http.FileServer(http.Dir(root)),
	}
}

func (h *devFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		h.files.ServeHTTP(w, r)
		return
	}

	rel := r.URL.Path
	if !strings.HasPrefix(rel, "/") {
		rel = "/" + rel
	}

	ext := filepath.Ext(rel)
	if ext != "" && ext != ".html" {
		h.files.ServeHTTP(w, r)
		return
	}

	candidates := []string{}
	if strings.HasSuffix(rel, "/") || rel == "" || ext == "" {
		base := rel
		if !strings.HasSuffix(base, "/") {
			base += "/"
		}
		candidates = append(candidates, base+"index.html")
		if ext == "" {
			candidates = append(candidates, rel+".html")
		}
	}
	if ext == ".html" {
		candidates = append(candidates, rel)
	}

	var data []byte
	var err error
	for _, candidate := range candidates {
		fullPath := filepath.Join(h.root, filepath.Clean(candidate))
		data, err = os.ReadFile(fullPath)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			http.Error(w, "error reading file", http.StatusInternalServerError)
			return
		}
	}
	if err != nil {
		if os.IsNotExist(err) && h.notFound != nil {
			h.serveNotFound(w, r)
			return
		}
		http.Error(w, "error reading file", http.StatusInternalServerError)
		return
	}

	injected := injectReloadScript(string(data))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(injected))
}

func (h *devFileHandler) serveNotFound(w http.ResponseWriter, r *http.Request) {
	var buf strings.Builder
	if err := h.notFound.ExecuteTemplate(&buf, "404", nil); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	injected := injectReloadScript(buf.String())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(injected))
}

func injectReloadScript(html string) string {
	snippet := `<script>
(() => {
  const es = new EventSource("/_shizuka/reload");
  es.onmessage = (event) => {
    if (event.data === "reload") {
      window.location.reload();
    }
  };
})();
</script>`

	lower := strings.ToLower(html)
	if idx := strings.LastIndex(lower, "</body>"); idx != -1 {
		return html[:idx] + snippet + html[idx:]
	}
	if idx := strings.LastIndex(lower, "</html>"); idx != -1 {
		return html[:idx] + snippet + html[idx:]
	}
	return html + snippet
}
