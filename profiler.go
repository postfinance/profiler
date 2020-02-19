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

// Hooker represents the interface for Profiler hooks
type Hooker interface {
	// PreStart will be executed after the signal was received but before the pprof endpoint starts
	PreStart()
	// PostShutdown will be executed after the pprof endpoint is shutdown
	PostShutdown()
}

// Profiler represents profiling
type Profiler struct {
	signal  os.Signal
	address string
	timeout time.Duration
	hooks   []Hooker

	stop chan struct{}
	done chan struct{}
	once *sync.Once
}

// Opt are Profiler functional options
type Opt func(*Profiler)

// WithSignal sets the signal to aktivate the pprof handler
func WithSignal(signal os.Signal) Opt {
	return func(p *Profiler) {
		p.signal = signal
	}
}

// WithAddress sets the listen address of the pprof handler
func WithAddress(address string) Opt {
	return func(p *Profiler) {
		p.address = address
	}
}

// WithTimeout sets the timeout after the pprof handler will be shutdown
func WithTimeout(timeout time.Duration) Opt {
	return func(p *Profiler) {
		p.timeout = timeout
	}
}

// WithHooks registers the Profiler hooks
func WithHooks(hooks ...Hooker) Opt {
	return func(p *Profiler) {
		p.hooks = append(p.hooks, hooks...)
	}
}

// New returns a new profiler
// Defaults:
// - Signal : syscall.SIGUSR1
// - Address: ":6666"
// - Timeout: 10m
func New(opts ...Opt) *Profiler {
	p := &Profiler{
		signal:  syscall.SIGUSR1,
		address: ":6666",
		timeout: 10 * time.Minute,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		once:    new(sync.Once),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Address returns the listen address for the pprof endpoint
func (p *Profiler) Address() string {
	return p.address
}

// Start the pprof signal handler
func (p *Profiler) Start() {
	go func() {
		p.once.Do(p.handler)
		p.once = new(sync.Once) // reset sync.Once for a subsequent call to Start
		log.Println("profiler handler stopped")
	}()
}

// Stop the pprof signal handler
func (p *Profiler) Stop() {
	p.stop <- struct{}{}
	<-p.done
}

func (p *Profiler) handler() {
	log.Printf("start profiler handler - pprof endpoint will be startet on signal: %v", p.signal)
	sig := make(chan os.Signal, 1)
	// stop receving signals and drain the signal channel

	for {
		// signal handling
		signal.Notify(sig, p.signal)
		select {
		case <-sig:
			disableSignals(sig)
		case <-p.stop:
			disableSignals(sig)
			p.done <- struct{}{}
			return
		}
		// start the pprof endpoint
		shutdown := make(chan struct{})
		srv := &http.Server{
			Addr:    p.address,
			Handler: pprofmux,
		}
		go func() {
			log.Printf("start pprof endpoint on %q\n", p.address)
			// execute the PreStart hooks
			for _, h := range p.hooks {
				h.PreStart()
			}
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Println("failed to start pprof endpoint:", err)
			} else {
				log.Println("pprof endpoint stopped")
			}
			/*
				err := srv.ListenAndServe()
				switch {
				case err != nil && err != http.ErrServerClosed:
					log.Println("failed to start pprof endpoint:", err)
				default:
					log.Println("pprof endpoint stopped")
				}
			*/
			// execute the PostShutdown hooks ... even after a failed startup
			for _, h := range p.hooks {
				h.PostShutdown()
			}
			close(shutdown)
		}()
		//
		timer := time.NewTimer(p.timeout)
		select {
		case <-timer.C: // timer expired
			shutdownEndpoint(srv, p.timeout)
			<-shutdown
		case <-shutdown: // start of endpoint failed
			if !timer.Stop() {
				<-timer.C
			}
		case <-p.stop: // stop requested
			if !timer.Stop() {
				<-timer.C
			}
			shutdownEndpoint(srv, p.timeout)
			<-shutdown
			p.done <- struct{}{}
			return
		}
	}
}

// disableSignals stop receiving of signals and drain the signal channel
func disableSignals(c chan os.Signal) {
	signal.Stop(c)
	// drain signal channel
	select {
	case <-c:
	default:
	}
}

// shutdownEndpoint shutdown the http server graceful
func shutdownEndpoint(srv *http.Server, timeout time.Duration) {
	log.Printf("shutdown pprof endpoint on %q\n", srv.Addr)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Println("failed to shutdown pprof endpoint:", err)
	}
}
