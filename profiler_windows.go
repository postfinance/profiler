// Package profiler implements functions to start a handler
package profiler

import (
	"syscall"
)

// New returns a new profiler
// Defaults:
// - Signal : syscall.SIGHUB
// - Address: ":6666"
// - Timeout: 30m
func New(options ...Option) *Profiler {
	p := Profiler{
		signal:  syscall.SIGHUP,
		address: defaultListen,
		timeout: defaultTimeout,

		evt: DefaultEventHandler(),
	}

	for _, option := range options {
		option(&p)
	}

	return &p
}
