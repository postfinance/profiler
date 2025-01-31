//go:build !windows

// Package profiler implements functions to start a handler
package profiler

import (
	"syscall"
)

// New returns a new profiler
// Defaults:
// - Signal : syscall.SIGUSR1
// - Address: ":6666"
// - Timeout: 30m
func New(options ...Option) *Profiler {
	p := Profiler{
		signal:  syscall.SIGUSR1,
		address: defaultListen,
		timeout: defaultTimeout,

		evt: DefaultEventHandler(),
	}

	for _, option := range options {
		option(&p)
	}

	return &p
}
