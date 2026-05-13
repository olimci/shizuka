//go:build pprof

package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
)

const defaultPprofAddr = "localhost:6060"

func init() {
	addr := os.Getenv("SHIZUKA_PPROF_ADDR")
	if addr == "" {
		addr = defaultPprofAddr
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "warning: pprof server %q: %v\n", addr, err)
		}
	}()
}
