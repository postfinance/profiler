// Package profiler implements functions to start a handler
package profiler

import (
	"context"
	"errors"
	"expvar"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/arl/statsviz"
)

const (
	defaultTimeout = 30 * time.Minute
	defaultListen  = ":6666"
)

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

	running sync.Mutex
	evt     EventHandler
}

// Address returns the listen address for the debug endpoint
func (p *Profiler) Address() string {
	return p.address
}

// =============================================================================

func (p *Profiler) Start(ctx context.Context) {
	if !p.running.TryLock() {
		return
	}

	go func() {
		defer p.running.Unlock()

		p.evt(InfoEvent, "start profiler signal handler", "signal", p.signal)

		sigC := make(chan os.Signal, 1)
		wg := new(sync.WaitGroup)
		ctx, cancel := context.WithCancel(ctx)

		for {
			signal.Notify(sigC, p.signal)

			select {
			case <-sigC: // receive signal to start the debug endpoint
				disableSignals(sigC)

				wg.Add(1)
				p.startEndpoint(ctx)
				wg.Done()

			case <-ctx.Done(): // stop the signal handler
				p.evt(InfoEvent, "stop profiler signal handler", "signal", p.signal)

				disableSignals(sigC)

				// stop the endpoint (if running) and
				// wait until the endpoint is stopped
				cancel()
				wg.Wait()

				p.evt(InfoEvent, "profiler signal handler stopped")

				return
			}
		}
	}()
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
		p.evt(InfoEvent, "start debug endpoint", "address", p.address)
		// execute the PreStart hooks
		for _, h := range p.hooks {
			h.PreStart()
		}

		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			p.evt(ErrorEvent, "start debug endpoint", "err", err)
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

	p.evt(InfoEvent, "stop debug endpoint", "address", p.address, "timeout", p.timeout)

	sCtx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	if err := srv.Shutdown(sCtx); err != nil {
		p.evt(ErrorEvent, "shutdown debug endpoint", "err", err)
	}

	<-shutdown
	p.evt(InfoEvent, "debug endpoint stopped")
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

	_ = statsviz.Register(mux)

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
