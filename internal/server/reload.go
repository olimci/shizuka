package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/felixge/httpsnoop"
)

func NewReloadHub() *ReloadHub {
	return &ReloadHub{
		clients: make(map[*ReloadClient]struct{}),
	}
}

type ReloadClient struct {
	Send chan string
}

type ReloadHub struct {
	mu      sync.RWMutex
	clients map[*ReloadClient]struct{}
}

func (h *ReloadHub) Broadcast(msg string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.Send <- msg:
		default:
		}
	}
}

func (h *ReloadHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	client := h.subscribe()
	defer h.unsubscribe(client)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case msg := <-client.Send:
			if _, err := fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
				return
			}
			flusher.Flush()
			if msg == "reload" {
				return
			}
		}
	}
}

func (h *ReloadHub) subscribe() *ReloadClient {
	client := &ReloadClient{Send: make(chan string, 8)}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client] = struct{}{}
	return client
}

func (h *ReloadHub) unsubscribe(client *ReloadClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.clients, client)
}

func ReloadMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !shouldInjectReload(r) {
			next.ServeHTTP(w, r)
			return
		}

		var body bytes.Buffer
		statusCode := http.StatusOK
		wroteHeader := false

		hooks := httpsnoop.Hooks{
			WriteHeader: func(original httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
				return func(code int) {
					statusCode = code
					wroteHeader = true
				}
			},
			Write: func(original httpsnoop.WriteFunc) httpsnoop.WriteFunc {
				return func(b []byte) (int, error) {
					body.Write(b)
					return len(b), nil
				}
			},
			ReadFrom: func(original httpsnoop.ReadFromFunc) httpsnoop.ReadFromFunc {
				return func(src io.Reader) (int64, error) {
					return io.Copy(&body, src)
				}
			},
		}

		wrapped := httpsnoop.Wrap(w, hooks)
		next.ServeHTTP(wrapped, r)

		contentType := w.Header().Get("Content-Type")
		if contentType == "" && body.Len() > 0 {
			contentType = http.DetectContentType(body.Bytes())
			if contentType != "" {
				w.Header().Set("Content-Type", contentType)
			}
		}
		if strings.Contains(contentType, "text/html") {
			injected := injectReloadScript(body.String())
			w.Header().Set("Content-Length", strconv.Itoa(len(injected)))
			if wroteHeader {
				w.WriteHeader(statusCode)
			}
			_, _ = w.Write([]byte(injected))
			return
		}

		if body.Len() > 0 {
			w.Header().Set("Content-Length", strconv.Itoa(body.Len()))
		}
		if wroteHeader {
			w.WriteHeader(statusCode)
		}
		_, _ = w.Write(body.Bytes())
	})
}

func injectReloadScript(html string) string {
	snippet := `<script>
(() => {
  const es = new EventSource("/_shizuka/reload");
  es.onmessage = (event) => {
    if (event.data === "reload") {
      es.close();
      window.location.reload();
    }
  };
  window.addEventListener("beforeunload", () => {
    es.close();
  });
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

func shouldInjectReload(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}

	ext := strings.ToLower(path.Ext(r.URL.Path))
	if ext != "" && ext != ".html" && ext != ".htm" {
		return false
	}

	accept := r.Header.Get("Accept")
	if accept == "" {
		return true
	}

	return strings.Contains(accept, "text/html")
}
