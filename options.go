package profiler

import (
	"os"
	"time"
)

// Option is a Profiler functional option
type Option func(*Profiler)

// WithSignal sets the signal to activate the pprof handler
func WithSignal(s os.Signal) Option {
	return func(p *Profiler) {
		p.signal = s
	}
}

// WithAddress sets the listen address of the pprof handler
func WithAddress(address string) Option {
	return func(p *Profiler) {
		p.address = address
	}
}

// WithTimeout sets the timeout after the pprof handler will be shutdown
func WithTimeout(timeout time.Duration) Option {
	return func(p *Profiler) {
		p.timeout = timeout
	}
}

// WithEventHandler registers a custom event handler
func WithEventHandler(evt EventHandler) Option {
	return func(p *Profiler) {
		p.evt = evt
	}
}

// WithHooks registers the Profiler hooks
func WithHooks(hooks ...Hooker) Option {
	return func(p *Profiler) {
		p.hooks = append(p.hooks, hooks...)
	}
}
