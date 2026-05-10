package profiler

import (
	"fmt"
	"net/http"
	"net/http/pprof"
)

const DefaultAddr = "localhost:8081"

type Profiler struct {
	server *http.Server
	addr   string
}

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

func (p *Profiler) Start() {
	go func() {
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("pprof server error: %v\n", err)
		}
	}()
}

func (p *Profiler) Addr() string {
	return p.addr
}

func (p *Profiler) Close() error {
	return p.server.Close()
}
