package internal

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/felixge/httpsnoop"
	"github.com/olimci/shizuka/pkg/utils/set"
)

func NewReloadClient() *ReloadClient {
	return &ReloadClient{
		Send: make(chan string, 8),
	}
}

type ReloadClient struct {
	Send chan string
}

func NewReloadHub() *ReloadHub {
	return &ReloadHub{
		clients: set.New[*ReloadClient](),
	}
}

type ReloadHub struct {
	mu      sync.RWMutex
	clients *set.Set[*ReloadClient]
}

func (h *ReloadHub) Broadcast(msg string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, client := range h.clients.Values() {
		select {
		case client.Send <- msg:
		default:
		}
	}
}

func (h *ReloadHub) Subscribe() *ReloadClient {
	client := NewReloadClient()

	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients.Add(client)

	return client
}

func (h *ReloadHub) Unsubscribe(client *ReloadClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients.Delete(client)
}

func (h *ReloadHub) Serve(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	client := h.Subscribe()
	defer h.Unsubscribe(client)

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-client.Send:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
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

func ReloadMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			w.Write([]byte(injected))
		} else {
			if body.Len() > 0 {
				w.Header().Set("Content-Length", strconv.Itoa(body.Len()))
			}
			if wroteHeader {
				w.WriteHeader(statusCode)
			}
			w.Write(body.Bytes())
		}
	})
}
