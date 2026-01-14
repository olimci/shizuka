package internal

import (
	"bufio"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type StaticHandlerOptions struct {
	HeadersFile   string
	RedirectsFile string
	NotFound      http.Handler
}

type StaticHandler struct {
	dist          string
	headersFile   string
	redirectsFile string
	notFound      http.Handler

	headersCache   cachedHeaders
	redirectsCache cachedRedirects
}

type cachedHeaders struct {
	mu      sync.RWMutex
	modTime time.Time
	ok      bool
	rules   []headerRule
}

type cachedRedirects struct {
	mu      sync.RWMutex
	modTime time.Time
	ok      bool
	rules   []redirectRule
}

type headerRule struct {
	pattern string
	headers map[string]string
}

type redirectRule struct {
	from   string
	to     string
	status int
}

func NewStaticHandler(dist string, opts StaticHandlerOptions) http.Handler {
	headersFile := opts.HeadersFile
	if strings.TrimSpace(headersFile) == "" {
		headersFile = "_headers"
	}

	redirectsFile := opts.RedirectsFile
	if strings.TrimSpace(redirectsFile) == "" {
		redirectsFile = "_redirects"
	}

	return &StaticHandler{
		dist:          dist,
		headersFile:   headersFile,
		redirectsFile: redirectsFile,
		notFound:      opts.NotFound,
	}
}

func (h *StaticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reqPath := normalizePath(r.URL.Path)
	headersPath := reqPath

	if h.isInternalControlPath(reqPath) {
		h.serveNotFound(w, r, headersPath, http.StatusNotFound)
		return
	}

	redirects := h.loadRedirects()
	if action, ok := matchRedirect(reqPath, redirects); ok {
		switch action.kind {
		case redirectActionRewrite:
			if !isExternalURL(action.target) {
				r = r.Clone(r.Context())
				if parsed, err := url.Parse(action.target); err == nil {
					r.URL.Path = parsed.Path
					r.URL.RawQuery = parsed.RawQuery
				} else {
					r.URL.Path = action.target
				}
				reqPath = normalizePath(r.URL.Path)
				break
			}
			action.kind = redirectActionRedirect
			action.status = http.StatusFound
		case redirectActionStatus:
			h.applyHeaders(w, headersPath)
			h.serveNotFound(w, r, headersPath, action.status)
			return
		case redirectActionRedirect:
			h.applyHeaders(w, headersPath)
			http.Redirect(w, r, action.target, action.status)
			return
		}
	}

	if filePath, redirectPath, ok := h.resolvePath(r.URL.Path); ok {
		if redirectPath != "" {
			h.applyHeaders(w, headersPath)
			http.Redirect(w, r, redirectPath, http.StatusMovedPermanently)
			return
		}

		h.applyHeaders(w, headersPath)
		http.ServeFile(w, r, filePath)
		return
	}

	h.serveNotFound(w, r, headersPath, http.StatusNotFound)
}

func (h *StaticHandler) applyHeaders(w http.ResponseWriter, reqPath string) {
	for _, rule := range h.loadHeaders() {
		if ok, _ := matchPattern(rule.pattern, reqPath); ok {
			for key, value := range rule.headers {
				w.Header().Set(key, value)
			}
		}
	}
}

func (h *StaticHandler) serveNotFound(w http.ResponseWriter, r *http.Request, headersPath string, status int) {
	customPath := filepath.Join(h.dist, "404.html")
	if info, err := os.Stat(customPath); err == nil && !info.IsDir() {
		h.applyHeaders(w, headersPath)
		sw := &statusWriter{ResponseWriter: w}
		sw.WriteHeader(status)
		http.ServeFile(sw, r, customPath)
		return
	}

	if h.notFound != nil {
		h.notFound.ServeHTTP(w, r)
		return
	}

	http.NotFound(w, r)
}

type statusWriter struct {
	http.ResponseWriter
	wrote bool
}

func (s *statusWriter) WriteHeader(code int) {
	if s.wrote {
		return
	}
	s.wrote = true
	s.ResponseWriter.WriteHeader(code)
}

func (h *StaticHandler) resolvePath(urlPath string) (string, string, bool) {
	clean := normalizePath(urlPath)
	rel := strings.TrimPrefix(clean, "/")
	fullPath := filepath.Join(h.dist, filepath.FromSlash(rel))

	info, err := os.Stat(fullPath)
	if err == nil {
		if info.IsDir() {
			if !strings.HasSuffix(urlPath, "/") {
				return "", clean + "/", true
			}
			indexPath := filepath.Join(fullPath, "index.html")
			if indexInfo, err := os.Stat(indexPath); err == nil && !indexInfo.IsDir() {
				return indexPath, "", true
			}
			return "", "", false
		}
		return fullPath, "", true
	}

	return "", "", false
}

func (h *StaticHandler) loadHeaders() []headerRule {
	filePath := filepath.Join(h.dist, filepath.FromSlash(h.headersFile))
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			h.headersCache.mu.Lock()
			h.headersCache.rules = nil
			h.headersCache.modTime = time.Time{}
			h.headersCache.ok = false
			h.headersCache.mu.Unlock()
		}
		return nil
	}

	h.headersCache.mu.RLock()
	if h.headersCache.ok && info.ModTime().Equal(h.headersCache.modTime) {
		rules := h.headersCache.rules
		h.headersCache.mu.RUnlock()
		return rules
	}
	h.headersCache.mu.RUnlock()

	rules, err := parseHeadersFile(filePath)
	if err != nil {
		h.headersCache.mu.RLock()
		cached := h.headersCache.rules
		h.headersCache.mu.RUnlock()
		return cached
	}

	h.headersCache.mu.Lock()
	h.headersCache.rules = rules
	h.headersCache.modTime = info.ModTime()
	h.headersCache.ok = true
	h.headersCache.mu.Unlock()

	return rules
}

func (h *StaticHandler) loadRedirects() []redirectRule {
	filePath := filepath.Join(h.dist, filepath.FromSlash(h.redirectsFile))
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			h.redirectsCache.mu.Lock()
			h.redirectsCache.rules = nil
			h.redirectsCache.modTime = time.Time{}
			h.redirectsCache.ok = false
			h.redirectsCache.mu.Unlock()
		}
		return nil
	}

	h.redirectsCache.mu.RLock()
	if h.redirectsCache.ok && info.ModTime().Equal(h.redirectsCache.modTime) {
		rules := h.redirectsCache.rules
		h.redirectsCache.mu.RUnlock()
		return rules
	}
	h.redirectsCache.mu.RUnlock()

	rules, err := parseRedirectsFile(filePath)
	if err != nil {
		h.redirectsCache.mu.RLock()
		cached := h.redirectsCache.rules
		h.redirectsCache.mu.RUnlock()
		return cached
	}

	h.redirectsCache.mu.Lock()
	h.redirectsCache.rules = rules
	h.redirectsCache.modTime = info.ModTime()
	h.redirectsCache.ok = true
	h.redirectsCache.mu.Unlock()

	return rules
}

func (h *StaticHandler) isInternalControlPath(reqPath string) bool {
	headersPath := "/" + strings.TrimPrefix(path.Clean("/"+h.headersFile), "/")
	redirectsPath := "/" + strings.TrimPrefix(path.Clean("/"+h.redirectsFile), "/")
	return reqPath == headersPath || reqPath == redirectsPath
}

type redirectActionKind string

const (
	redirectActionRedirect redirectActionKind = "redirect"
	redirectActionRewrite  redirectActionKind = "rewrite"
	redirectActionStatus   redirectActionKind = "status"
)

type redirectAction struct {
	kind   redirectActionKind
	target string
	status int
}

func matchRedirect(reqPath string, rules []redirectRule) (redirectAction, bool) {
	for _, rule := range rules {
		matched, splat := matchPattern(rule.from, reqPath)
		if !matched {
			continue
		}

		target := rule.to
		if splat != "" {
			target = strings.ReplaceAll(target, ":splat", splat)
			target = strings.ReplaceAll(target, "*", splat)
		}
		if target != "" && !strings.HasPrefix(target, "/") && !strings.HasPrefix(target, "?") && !strings.HasPrefix(target, "#") && !isExternalURL(target) {
			target = "/" + target
		}

		status := rule.status
		if status == 0 {
			status = http.StatusFound
		}

		if status == http.StatusOK {
			return redirectAction{kind: redirectActionRewrite, target: ensureLeadingSlash(target), status: status}, true
		}

		if status == http.StatusNotFound || status == http.StatusGone {
			return redirectAction{kind: redirectActionStatus, status: status}, true
		}

		return redirectAction{kind: redirectActionRedirect, target: target, status: status}, true
	}

	return redirectAction{}, false
}

func normalizePath(raw string) string {
	clean := path.Clean("/" + raw)
	if clean == "." {
		return "/"
	}
	if !strings.HasPrefix(clean, "/") {
		return "/" + clean
	}
	return clean
}

func ensureLeadingSlash(target string) string {
	if target == "" {
		return "/"
	}
	if strings.HasPrefix(target, "/") || strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target
	}
	return "/" + target
}

func isExternalURL(target string) bool {
	parsed, err := url.Parse(target)
	if err != nil {
		return false
	}
	return parsed.Scheme != "" && parsed.Host != ""
}

func matchPattern(pattern, value string) (bool, string) {
	pattern = normalizePath(pattern)
	value = normalizePath(value)

	if pattern == value {
		return true, ""
	}

	if !strings.Contains(pattern, "*") {
		return false, ""
	}

	parts := strings.Split(pattern, "*")
	if len(parts) != 2 {
		return false, ""
	}

	prefix := parts[0]
	suffix := parts[1]

	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, suffix) {
		return false, ""
	}

	splat := strings.TrimSuffix(strings.TrimPrefix(value, prefix), suffix)
	return true, splat
}

func parseHeadersFile(filePath string) ([]headerRule, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	rules := make([]headerRule, 0)
	current := -1

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			current = -1
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}

		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			if current < 0 {
				continue
			}
			line = strings.TrimSpace(line)
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			rules[current].headers[key] = value
			continue
		}

		pattern := strings.TrimSpace(line)
		rules = append(rules, headerRule{
			pattern: pattern,
			headers: map[string]string{},
		})
		current = len(rules) - 1
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return rules, nil
}

func parseRedirectsFile(filePath string) ([]redirectRule, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	rules := make([]redirectRule, 0)

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		from := fields[0]
		if parsed, err := url.Parse(from); err == nil && parsed.Host != "" {
			from = parsed.Path
		}
		from = normalizePath(from)
		to := fields[1]
		status := 0

		if len(fields) > 2 {
			if parsed, err := strconv.Atoi(fields[2]); err == nil {
				status = parsed
			}
		}

		rules = append(rules, redirectRule{
			from:   from,
			to:     to,
			status: status,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return rules, nil
}
