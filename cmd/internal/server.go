package internal

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

type Server struct {
	server *http.Server
	addr   string
}

type ServerConfig struct {
	DistDir string
	Port    int
}

func NewServer(config ServerConfig) *Server {
	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir(config.DistDir))
	mux.Handle("/", fs)

	addr := fmt.Sprintf("127.0.0.1:%d", config.Port)

	return &Server{
		server: &http.Server{
			Handler: mux,
		},
		addr: addr,
	}
}

func (s *Server) Start(ctx context.Context) (string, error) {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return "", fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}

	go func() {
		_ = s.server.Serve(ln)
	}()

	go func() {
		<-ctx.Done()
		_ = s.server.Close()
	}()

	baseURL := "http://" + s.addr + "/"
	return baseURL, nil
}

func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}
