// Package profiler runs a separate HTTP server for pprof debugging endpoints.
package profiler

import (
	"fmt"
	"net/http"
	"net/http/pprof"
)

// DefaultAddr is the default pprof server address.
const DefaultAddr = "localhost:8081"

// Profiler serves pprof debugging endpoints on a separate HTTP server.
type Profiler struct {
	server *http.Server
	addr   string
}

// New creates a Profiler with pprof handlers registered on the default address.
func New() *Profiler {
	addr := DefaultAddr

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))

	return &Profiler{
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
		addr: addr,
	}
}

// Start launches the pprof HTTP server in a background goroutine.
func (p *Profiler) Start() {
	go func() {
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("pprof server error: %v\n", err)
		}
	}()
}

// Addr returns the address the pprof server is listening on.
func (p *Profiler) Addr() string {
	return p.addr
}

// Close shuts down the pprof HTTP server.
func (p *Profiler) Close() error {
	return p.server.Close()
}
