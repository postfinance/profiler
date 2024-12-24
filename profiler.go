// Package profiler implements functions to start a handler
package profiler

import (
	"context"
	"errors"
	"expvar"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"net/http/pprof"
)

// EventHandler function to handle events
type EventHandler func(v string, args ...any)

// Hooker represents the interface for Profiler hooks
type Hooker interface {
	// PreStart will be executed after the signal was received but before the debug endpoint starts
	PreStart()
	// PostShutdown will be executed after the debug endpoint is shutdown or the start has failed
	PostShutdown()
}

// =============================================================================

// Profiler represents the Profiler
type Profiler struct {
	signal  os.Signal
	address string
	timeout time.Duration
	hooks   []Hooker

	started *atomic.Int32
	running sync.Mutex
	stopC   chan struct{}
	evt     EventHandler
}

// New returns a new profiler
// Defaults:
// - Signal : syscall.SIGHUP
// - Address: ":6666"
// - Timeout: 10m
func New(options ...Option) *Profiler {
	p := &Profiler{
		signal:  syscall.SIGHUP,
		address: ":6666",
		timeout: 10 * time.Minute,

		started: new(atomic.Int32),
		stopC:   make(chan struct{}),
		evt: func(msg string, args ...any) {
			log.Println(append([]any{msg}, args...)...)
		},
	}

	for _, option := range options {
		option(p)
	}

	return p
}

// Address returns the listen address for the debug endpoint
func (p *Profiler) Address() string {
	return p.address
}

// Start the profiler signal handler
// After the first call, subsequent calls
// to Start do nothing until Stop is called.
func (p *Profiler) Start() {
	if p.started.CompareAndSwap(0, 1) {
		go p.start()
	}
}

// Stop the profiler signal handler
// After the first call, subsequent calls
// to Stop do nothing until Start is called.
func (p *Profiler) Stop() {
	if p.started.CompareAndSwap(1, 0) {
		p.stopC <- struct{}{}
	}
}

// =============================================================================

func (p *Profiler) start() {
	p.running.Lock()
	defer p.running.Unlock()

	p.evt("start profiler signal handler", "signal", p.signal)
	defer p.evt("profiler signal handler stopped")

	sigC := make(chan os.Signal, 1)
	wg := new(sync.WaitGroup)
	ctx, cancel := context.WithCancel(context.Background())

	for {
		signal.Notify(sigC, p.signal)

		select {
		case <-sigC: // receive signal to start the debug endpoint
			disableSignals(sigC)

			wg.Add(1)
			p.startEndpoint(ctx)
			wg.Done()
		case <-p.stopC: // stop the signal handler
			p.evt("stop profiler signal handler", "signal", p.signal)

			disableSignals(sigC)

			// stop the endpoint (if running) and
			// wait until the endpoint is stopped
			cancel()
			wg.Wait()

			return
		}
	}
}

// startEndpoint starts the debug http endpoint
func (p *Profiler) startEndpoint(ctx context.Context) {
	shutdown := make(chan struct{})

	srv := &http.Server{
		Addr:         p.address,
		Handler:      standardLibraryMux(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		p.evt("start debug endpoint", "address", p.address)
		defer p.evt("debug endpoint stopped")
		// execute the PreStart hooks
		for _, h := range p.hooks {
			h.PreStart()
		}

		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			p.evt("ERROR: start debug endpoint", "err", err)
		} else {
			p.evt("debug endpoint stopped")
		}

		// execute the PostShutdown hooks ... even after a failed startup
		for _, h := range p.hooks {
			h.PostShutdown()
		}

		close(shutdown)
	}()

	timer := time.NewTimer(p.timeout)

	select {
	case <-timer.C: // timer expired
	case <-ctx.Done(): // context canceled
		timer.Stop()
	}

	p.evt("shutdown debug endpoint", "address", p.address, "timeout", p.timeout)

	sCtx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	if err := srv.Shutdown(sCtx); err != nil {
		p.evt("ERROR: shutdown debug endpoint", "err", err)
	}

	<-shutdown
}

// =============================================================================

// standardLibraryMux registers all the debug routes from the standard library
// into a new mux bypassing the use of the DefaultServerMux. Using the
// DefaultServerMux would be a security risk since a dependency could inject a
// handler into our service without us knowing it.
//
// Source: https://github.com/ardanlabs/service4.1-video/blob/main/business/web/v1/debug/debug.go
func standardLibraryMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/debug/vars", expvar.Handler())

	return mux
}

// disableSignals stop receiving of signals and drain the signal channel
func disableSignals(sigC chan os.Signal) {
	signal.Stop(sigC)

	// drain signal channel
	select {
	case <-sigC:
	default:
	}
}
