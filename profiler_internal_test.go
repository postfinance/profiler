package profiler

import (
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultProfiler(t *testing.T) {
	p := New()
	assert.Equal(t, syscall.SIGUSR1, p.signal)
	assert.Equal(t, ":6666", p.address)
	assert.Equal(t, 10*time.Minute, p.timeout)
}

func TestWithSignal(t *testing.T) {
	signal := syscall.SIGUSR2
	p := New(WithSignal(signal))
	assert.Equal(t, signal, p.signal)
}

func TestWithAddress(t *testing.T) {
	address := ":8080"
	p := New(WithAddress(address))
	assert.Equal(t, address, p.address)
}

func TestWithTimeout(t *testing.T) {
	timeout := 5 * time.Minute
	p := New(WithTimeout(timeout))
	assert.Equal(t, timeout, p.timeout)
}
