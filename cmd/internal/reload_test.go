package internal

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInjectReloadScript(t *testing.T) {
	html := "<html><body><h1>Hi</h1></body></html>"
	got := injectReloadScript(html)

	if !strings.Contains(got, `new EventSource("/_shizuka/reload")`) {
		t.Fatalf("injectReloadScript() = %q, want reload script", got)
	}
	if !strings.Contains(got, "</script></body>") {
		t.Fatalf("injectReloadScript() did not inject before </body>: %q", got)
	}
}

func TestShouldInject(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	req.Header.Set("Accept", "text/html")
	if !shouldInject(req) {
		t.Fatal("shouldInject(html GET) = false, want true")
	}

	req = httptest.NewRequest(http.MethodGet, "/style.css", nil)
	if shouldInject(req) {
		t.Fatal("shouldInject(css) = true, want false")
	}

	req = httptest.NewRequest(http.MethodPost, "/", nil)
	if shouldInject(req) {
		t.Fatal("shouldInject(POST) = true, want false")
	}
}

func TestReloadMiddlewareInjectsOnlyHTML(t *testing.T) {
	htmlHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("<html><body>Hello</body></html>"))
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ReloadMiddleware(htmlHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("HTML response status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if !strings.Contains(rec.Body.String(), `new EventSource("/_shizuka/reload")`) {
		t.Fatalf("HTML response missing reload script: %q", rec.Body.String())
	}

	textHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("plain"))
	})

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	ReloadMiddleware(textHandler).ServeHTTP(rec, req)

	if rec.Body.String() != "plain" {
		t.Fatalf("plain response = %q, want %q", rec.Body.String(), "plain")
	}
}
