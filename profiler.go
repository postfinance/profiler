// Package profiler implements functions to start a handler
package profiler

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "net/http/pprof" // normally pprof will be imported in the main package
)

var (
	pprofmux *http.ServeMux
)

func init() {
	pprofmux = http.DefaultServeMux
	http.DefaultServeMux = http.NewServeMux()
}

// Profiler represents profiling
type Profiler struct {
	sig     os.Signal
	addr    string
	timeout time.Duration
	once    sync.Once
}

// Opt are Profiler functional options
type Opt func(*Profiler)

// WithSignal sets the signal to aktivate the pprof handler
func WithSignal(sig os.Signal) Opt {
	return func(p *Profiler) {
		p.sig = sig
	}
}

// WithAddress sets the listen address of the pprof handler
func WithAddress(addr string) Opt {
	return func(p *Profiler) {
		p.addr = addr
	}
}

// WithTimeout sets the timeout after the pprof handler will be shutdown
func WithTimeout(timeout time.Duration) Opt {
	return func(p *Profiler) {
		p.timeout = timeout
	}
}

// New returns a new profiler
// Defaults:
// - Signal : syscall.SIGUSR1
// - Address: ":6666"
// - Timeout: 10m
func New(opts ...Opt) *Profiler {
	p := &Profiler{
		sig:     syscall.SIGUSR1,
		addr:    ":6666",
		timeout: 10 * time.Minute,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Start the pprof signal handler
func (p *Profiler) Start() {
	go func() {
		p.once.Do(p.handler)
	}()
}

func (p *Profiler) handler() {
	log.Printf("pprof endpoint will be startet on signal: %v", p.sig)
	sigs := make(chan os.Signal, 1)
	for {
		signal.Notify(sigs, p.sig)
		<-sigs
		signal.Stop(sigs)
		// clear channel if necessary
		select {
		case <-sigs:
		default:
		}
		log.Printf("start pprof endpoint on %q", p.addr)
		shutdown := make(chan struct{})
		srv := &http.Server{
			Addr:    p.addr,
			Handler: pprofmux,
		}
		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Println("failed to start pprof endpoint:", err)
			}
			log.Println("pprof endpoint stopped")
			close(shutdown)
		}()
		<-time.NewTimer(p.timeout).C
		log.Printf("shutdown pprof endpoint on %q", p.addr)
		ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
		if err := srv.Shutdown(ctx); err != nil {
			log.Println("failed to shutdown pprof endpoint:", err)
		}
		cancel()
		<-shutdown
	}
}
